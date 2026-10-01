import { motion } from 'motion/react'
import type { ReactNode } from 'react'

import { Logo } from '@/components/logo'
import { MascotArt } from '@/components/mascot'
import { useMascotArt } from '@/hooks/use-mascot-art'

export interface AuthCardProps {
  title: string
  subtitle?: ReactNode
  children: ReactNode
}

/**
 * Centered glass card with the logo, used by the login and setup pages. On wide screens the
 * mascot waves hello beside it.
 */
export function AuthCard({ title, subtitle, children }: AuthCardProps) {
  const showArt = useMascotArt()
  return (
    <div className="relative w-full max-w-[400px]">
      {showArt ? (
        <motion.div
          initial={{ opacity: 0, x: -12 }}
          animate={{ opacity: 1, x: 0 }}
          transition={{ delay: 0.15 }}
          className="absolute bottom-4 left-full ml-4 hidden w-max lg:block"
        >
          <MascotArt pose="welcome" className="h-64 drop-shadow-sm" />
        </motion.div>
      ) : null}
      <motion.section
        initial={{ opacity: 0, y: 16, scale: 0.98 }}
        animate={{ opacity: 1, y: 0, scale: 1 }}
        transition={{ type: 'spring', stiffness: 400, damping: 36 }}
        className="w-full rounded-[28px] border border-white/60 bg-background/70 p-6 shadow-2xl shadow-black/[0.07] backdrop-blur-2xl backdrop-saturate-150 sm:p-8 dark:border-white/10 dark:bg-background/55 dark:shadow-black/40"
      >
        <div className="mb-7 flex flex-col items-center text-center">
          <Logo size={60} className="mb-5 drop-shadow-md" />
          <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
          {subtitle ? <p className="mt-1.5 text-sm text-balance text-muted-foreground">{subtitle}</p> : null}
        </div>
        {children}
      </motion.section>
    </div>
  )
}
