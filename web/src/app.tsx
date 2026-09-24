import { QueryClientProvider } from '@tanstack/react-query'
import { MotionConfig } from 'motion/react'
import { RouterProvider } from 'react-router/dom'

import { ThemeProvider } from '@/components/theme-provider'
import { TooltipProvider } from '@/components/ui/tooltip'
import { queryClient } from '@/lib/query-client'
import { router } from '@/router'

/** App providers. i18n is initialised by importing `@/lib/i18n` (see main.tsx). */
export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>
        <MotionConfig reducedMotion="user" transition={{ type: 'spring', stiffness: 400, damping: 36 }}>
          <TooltipProvider delayDuration={400}>
            <RouterProvider router={router} />
          </TooltipProvider>
        </MotionConfig>
      </ThemeProvider>
    </QueryClientProvider>
  )
}
