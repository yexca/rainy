/** Scan status + start-scan hooks shared by the libraries page. */
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { toastError } from '@/features/manage/lib/batch'
import { useServerEvent } from '@/hooks/use-server-events'
import { isApiError } from '@/lib/api/client'
import { api } from '@/lib/api/endpoints'
import type { ScanStatus } from '@/lib/api/types'

import { adminKeys } from './queries'

const POLL_MS = 2000

function isScanStatus(data: unknown): data is ScanStatus {
  return typeof data === 'object' && data !== null && 'phase' in data && 'scanning' in data
}

/** Scan status query: live via `scan` server events, polling as a fallback while scanning. */
export function useScanStatus() {
  const queryClient = useQueryClient()
  const query = useQuery({
    queryKey: adminKeys.scan,
    queryFn: ({ signal }) => api.admin.scan.status({ signal }),
    refetchInterval: (q) => (q.state.data?.scanning ? POLL_MS : false),
  })
  useServerEvent('scan', (data) => {
    if (isScanStatus(data)) queryClient.setQueryData(adminKeys.scan, data)
  })
  return query
}

/** Start a scan (409 → "already running" toast). */
export function useStartScan() {
  const { t } = useTranslation('admin')
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: { full?: boolean; libraryId?: number }) => api.admin.scan.start(input),
    onSuccess: (status) => {
      if (isScanStatus(status)) queryClient.setQueryData(adminKeys.scan, status)
      else void queryClient.invalidateQueries({ queryKey: adminKeys.scan })
      toast.success(t('scan.started'))
    },
    onError: (error) => {
      if (isApiError(error) && error.status === 409) {
        toast(t('scan.alreadyRunning'))
        void queryClient.invalidateQueries({ queryKey: adminKeys.scan })
      } else toastError(error)
    },
  })
}
