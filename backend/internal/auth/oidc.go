package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// Claims คือข้อมูลจาก ID token ที่แอปใช้
type Claims struct {
	Issuer, Subject   string
	Nonce             string
	Name              string
	PreferredUsername string
	GivenName         string
}

// Exchanger แลก authorization code เป็น ID token ที่ตรวจแล้ว
// แยกเป็น interface เพื่อให้ test ใช้ตัวปลอมได้โดยไม่ต้องมี Keycloak
type Exchanger interface {
	Exchange(ctx context.Context, code, verifier string) (Claims, string, error)
}

// OIDC คือ client ของ Keycloak (confidential client + PKCE)
type OIDC struct {
	OAuth         oauth2.Config
	EndSessionURL string
	verifier      *oidc.IDTokenVerifier
}

// NewOIDC อ่าน discovery document ของ issuer ตอนเริ่มโปรแกรม
// redirect URI สร้างจาก PUBLIC_BASE_URL เสมอ ไม่ใช้ header Host
func NewOIDC(ctx context.Context, issuer, clientID, clientSecret, publicBaseURL string) (*OIDC, error) {
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("ต่อ Keycloak ที่ %s ไม่ได้ (รัน docker compose up -d แล้วหรือยัง?): %w", issuer, err)
	}
	var meta struct {
		EndSession string `json:"end_session_endpoint"`
	}
	if err := provider.Claims(&meta); err != nil {
		return nil, err
	}
	if meta.EndSession == "" {
		return nil, errors.New("discovery document ไม่มี end_session_endpoint")
	}
	return &OIDC{
		OAuth: oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Endpoint:     provider.Endpoint(),
			RedirectURL:  publicBaseURL + "/api/auth/callback",
			Scopes:       []string{oidc.ScopeOpenID, "profile"},
		},
		EndSessionURL: meta.EndSession,
		verifier:      provider.Verifier(&oidc.Config{ClientID: clientID}),
	}, nil
}

// Exchange แลก code ด้วย client secret + PKCE verifier แล้วตรวจ ID token
// (ลายเซ็น, iss, aud, exp โดย go-oidc) ส่วน nonce ผู้เรียกต้องเทียบเองเพราะ go-oidc ไม่ตรวจให้
func (o *OIDC) Exchange(ctx context.Context, code, verifier string) (Claims, string, error) {
	tok, err := o.OAuth.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return Claims{}, "", err
	}
	raw, ok := tok.Extra("id_token").(string)
	if !ok {
		return Claims{}, "", errors.New("token response ไม่มี id_token")
	}
	idt, err := o.verifier.Verify(ctx, raw)
	if err != nil {
		return Claims{}, "", err
	}
	var extra struct {
		Name              string `json:"name"`
		PreferredUsername string `json:"preferred_username"`
		GivenName         string `json:"given_name"`
	}
	if err := idt.Claims(&extra); err != nil {
		return Claims{}, "", err
	}
	return Claims{
		Issuer:            idt.Issuer,
		Subject:           idt.Subject,
		Nonce:             idt.Nonce,
		Name:              extra.Name,
		PreferredUsername: extra.PreferredUsername,
		GivenName:         extra.GivenName,
	}, raw, nil
}
