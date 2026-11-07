import { useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'
import { toast } from 'sonner'

import type { DownloadJob } from '@/lib/api/types'

import { invalidateLibrary, manageKeys } from '../queries'
import { isActiveJob } from './downloads'
import { promptUntagged } from './untagged'

/**
 * Toasts when a download job this page saw running finishes, asks for the tags of tracks that
 * arrived without an artist or album, and refreshes the library (or, for a job that held its
 * files for review, the Review list).
 */
export function useJobNotifications(jobs: DownloadJob[] | undefined) {
  const { t } = useTranslation('manage')
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const seen = useRef(new Map<string, DownloadJob['status']>())
  useEffect(() => {
    if (!jobs) return
    let changed = false
    let staged = false
    const untagged: string[] = []
    for (const job of jobs) {
      const before = seen.current.get(job.id)
      seen.current.set(job.id, job.status)
      if (!before || !isActiveJob({ status: before }) || isActiveJob(job)) continue
      const name = job.online ? job.online.song.title : job.title || job.url
      if (job.status === 'done' && job.stagedIds?.length) {
        staged = true
        toast.success(t('review.downloaded', { name, count: job.stagedIds.length }), {
          description: job.error || undefined,
          action: { label: t('review.open'), onClick: () => void navigate('/manage/review') },
        })
      } else if (job.status === 'done') {
        changed = true
        untagged.push(...(job.untagged ?? []))
        if (job.error) toast.warning(t('download.finishedPartly', { name, count: job.trackIds.length }), { description: job.error })
        else toast.success(t('download.finished', { name, count: job.trackIds.length }))
      } else if (job.status === 'error') {
        toast.error(t('download.failed', { name }), { description: job.error })
      }
    }
    promptUntagged(untagged)
    if (changed) void invalidateLibrary(queryClient)
    if (staged) void queryClient.invalidateQueries({ queryKey: manageKeys.staging })
  }, [jobs, navigate, queryClient, t])
}
