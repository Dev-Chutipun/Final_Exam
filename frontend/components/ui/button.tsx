import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"
import { cn } from "cn"
import { Slot } from "radix-ui"

// Variant ตาม docs/05-frontend.md §6 (พาสเทลกระจก) · ตัวอักษรเข้มทุกปุ่ม เพราะพาสเทลรองรับตัวอักษรขาวไม่ได้
// สีมาจาก token ใน app/globals.css เท่านั้น · outline/ghost เก็บไว้เพราะ dialog ของ shadcn ใช้อยู่
const buttonVariants = cva(
  "group/button font-heading inline-flex shrink-0 items-center justify-center gap-2 rounded-(--r-btn) border border-(--border-glass) font-semibold whitespace-nowrap backdrop-blur-[14px] backdrop-saturate-[1.7] transition-all outline-none select-none active:not-aria-[haspopup]:translate-y-px disabled:pointer-events-none disabled:border-white/75 disabled:bg-white/45 disabled:bg-none disabled:text-(--ink-disabled) disabled:shadow-none aria-invalid:border-(--invalid) [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
  {
    variants: {
      variant: {
        default:
          "bg-linear-to-b from-(--btn-primary-from) to-(--btn-primary-to) text-(--ink) shadow-[inset_0_1px_0_#fff,inset_0_-1px_0_rgba(107,91,214,0.18),0_8px_22px_rgba(107,91,214,0.22)] hover:brightness-[1.03]",
        cta: "bg-linear-to-b from-(--btn-cta-from) to-(--btn-cta-to) text-(--btn-cta-ink) font-bold shadow-[inset_0_1px_0_#fff,inset_0_-1px_0_rgba(214,110,80,0.2),0_10px_26px_rgba(255,140,105,0.30)] hover:brightness-[1.03]",
        glass:
          "bg-linear-to-b from-(--btn-glass-from) to-(--btn-glass-to) text-(--ink) shadow-[inset_0_1px_0_#fff,0_6px_16px_rgba(107,91,214,0.10)] hover:from-white",
        danger:
          "bg-linear-to-b from-(--btn-danger-from) to-(--btn-danger-to) text-(--btn-danger-ink) shadow-[inset_0_1px_0_#fff,0_6px_16px_rgba(214,80,110,0.16)] hover:brightness-[1.02]",
        "danger-strong":
          "bg-linear-to-b from-(--btn-danger-strong-from) to-(--btn-danger-strong-to) text-(--btn-danger-strong-ink) shadow-[inset_0_1px_0_#fff,inset_0_-1px_0_rgba(214,80,110,0.25),0_8px_22px_rgba(214,80,110,0.25)] hover:brightness-[1.02]",
        outline:
          "bg-linear-to-b from-(--btn-glass-from) to-(--btn-glass-to) text-(--ink) shadow-[inset_0_1px_0_#fff,0_6px_16px_rgba(107,91,214,0.10)]",
        ghost: "border-transparent bg-transparent text-(--ink) shadow-none backdrop-blur-none hover:bg-white/60",
      },
      size: {
        default: "min-h-12 px-[22px] text-base",
        sm: "min-h-11 px-4 text-[15px]",
        lg: "min-h-14 rounded-[18px] px-7 text-lg",
        confirm: "min-h-[58px] rounded-[18px] px-7 text-lg font-bold",
        icon: "size-11",
        "icon-sm": "size-9 rounded-xl",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  }
)

function Button({
  className,
  variant = "default",
  size = "default",
  asChild = false,
  ...props
}: React.ComponentProps<"button"> &
  VariantProps<typeof buttonVariants> & {
    asChild?: boolean
  }) {
  const Comp = asChild ? Slot.Root : "button"

  return (
    <Comp
      data-slot="button"
      data-variant={variant}
      data-size={size}
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  )
}

export { Button, buttonVariants }
