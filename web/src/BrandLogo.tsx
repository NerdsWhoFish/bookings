import type { ComponentPropsWithoutRef } from 'react'

export function BrandLogo({ className, ...props }: ComponentPropsWithoutRef<'a'>) {
  const classes = ['wordmark', className].filter(Boolean).join(' ')

  return <a {...props} className={classes}>
    <img className="wordmark-logo" src="/assets/brand/nwf-logo.svg" alt="Nerds Who Fish, Software Solutions" />
  </a>
}
