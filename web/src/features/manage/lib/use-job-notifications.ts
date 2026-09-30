import { useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import type { DownloadJob } from '@/lib/api/types'

import { invalidateLibrary } from '../queries'
import { isActiveJob } from './downloads'

/** Toasts when a download job this page saw running finishes, and refreshes the library. */
export function useJobNotifications(jobs: DownloadJob[] | undefined) {
  const { t } = useTranslation('manage')
  const queryClient = useQueryClient()
  const seen = useRef(new Map<string, DownloadJob['status']>())
  useEffect(() => {
    if (!jobs) return
    let changed = false
    for (const job of jobs) {
      const before = seen.current.get(job.id)
      seen.current.set(job.id, job.status)
      if (!before || !isActiveJob({ status: before }) || isActiveJob(job)) continue
      const name = job.online ? job.online.song.title : job.title || job.url
      if (job.status === 'done') {
        changed = true
        if (job.error) toast.warning(t('download.finishedPartly', { name, count: job.trackIds.length }), { description: job.error })
        else toast.success(t('download.finished', { name, count: job.trackIds.length }))
      } else if (job.status === 'error') {
        toast.error(t('download.failed', { name }), { description: job.error })
      }
    }
    if (changed) void invalidateLibrary(queryClient)
  }, [jobs, queryClient, t])
}
