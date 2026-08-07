import type { PropsWithChildren } from 'react'

export function StatusPill({ children }: PropsWithChildren) {
  return (
    <span className="inline-flex items-center gap-2 rounded-full border border-emerald-300/20 bg-emerald-300/10 px-3 py-1 text-sm text-emerald-200">
      <span className="size-2 rounded-full bg-emerald-300 shadow-[0_0_14px_rgba(110,231,183,0.8)]" />
      {children}
    </span>
  )
}

