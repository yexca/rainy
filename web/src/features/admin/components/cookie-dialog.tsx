import { useMutation, useQueryClient } from '@tanstack/react-query'
import { ExternalLink, FileText, ShieldAlert } from 'lucide-react'
import { useRef, useState } from 'react'
import { Trans, useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Spinner } from '@/components/spinner'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { ResponsiveDialog } from '@/features/manage/components/responsive-dialog'
import { api } from '@/lib/api/endpoints'
import type { DownloadSite } from '@/lib/api/types'
import { errorMessage } from '@/lib/errors'

import { adminKeys } from '../queries'

/** The browser extension recommended for exporting cookies.txt (runs locally, no upload). */
export const COOKIE_EXTENSION_URL = 'https://chromewebstore.google.com/detail/get-cookiestxt-locally/cclelndahbckbenkjhflpdbgdldlbecc'

const MAX_FILE = 512 * 1024

export interface CookieDialogProps {
  site: DownloadSite | null
  onOpenChange: (open: boolean) => void
}

/**
 * Adds sign-in cookies for yt-dlp. The first step is a warning the admin must acknowledge:
 * cookies are account credentials. The text is sent once to this server, which keeps only the
 * site's own cookies, encrypted; it is never shown again and never stored in the browser.
 */
export function CookieDialog({ site, onOpenChange }: CookieDialogProps) {
  const { t } = useTranslation('admin')
  const [acknowledged, setAcknowledged] = useState(false)
  const [text, setText] = useState('')
  const fileRef = useRef<HTMLInputElement>(null)
  const queryClient = useQueryClient()

  const close = (open: boolean) => {
    if (!open) {
      // Nothing of the cookies stays in memory once the dialog is gone.
      setText('')
      setAcknowledged(false)
    }
    onOpenChange(open)
  }

  const save = useMutation({
    mutationFn: () => api.admin.ytdlp.setCookies(site!, text),
    onSuccess: (res) => {
      void queryClient.invalidateQueries({ queryKey: adminKeys.ytdlp })
      void queryClient.invalidateQueries({ queryKey: ['manage', 'downloads'] })
      const description = [
        res.dropped > 0 ? t('ytdlp.cookies.dropped', { count: res.dropped }) : '',
        res.signedIn ? '' : t('ytdlp.cookies.notSignedInHint'),
      ]
        .filter(Boolean)
        .join(' ')
      if (res.signedIn) toast.success(t('ytdlp.cookies.saved', { count: res.count }), { description: description || undefined })
      else toast.warning(t('ytdlp.cookies.saved', { count: res.count }), { description })
      close(false)
    },
    onError: (error) => toast.error(errorMessage(error, t)),
  })

  const siteName = site ? t(`ytdlp.sites.${site}`) : ''
  const domain = site === 'youtube' ? 'youtube.com' : 'bilibili.com'

  if (!acknowledged) {
    return (
      <ResponsiveDialog
        open={site !== null}
        onOpenChange={close}
        title={
          <span className="flex items-center gap-2">
            <ShieldAlert className="size-5 shrink-0 text-destructive" strokeWidth={1.75} aria-hidden />
            {t('ytdlp.warning.title')}
          </span>
        }
        size="sm"
        footer={
          <>
            <Button variant="outline" onClick={() => close(false)}>
              {t('common:actions.cancel')}
            </Button>
            <Button variant="destructive" onClick={() => setAcknowledged(true)}>
              {t('ytdlp.warning.accept')}
            </Button>
          </>
        }
      >
        <div className="grid gap-3 text-sm">
          <p className="font-medium text-destructive">{t('ytdlp.warning.body')}</p>
          <p>{t('ytdlp.warning.never')}</p>
          <p className="text-muted-foreground">{t('ytdlp.warning.local')}</p>
        </div>
      </ResponsiveDialog>
    )
  }

  return (
    <ResponsiveDialog
      open={site !== null}
      onOpenChange={close}
      title={t('ytdlp.cookies.dialogTitle', { site: siteName })}
      description={t('ytdlp.cookies.dialogDescription')}
      size="lg"
      dismissible={!save.isPending}
      footer={
        <>
          <Button variant="outline" onClick={() => close(false)} disabled={save.isPending}>
            {t('common:actions.cancel')}
          </Button>
          <Button onClick={() => save.mutate()} disabled={save.isPending || text.trim() === ''}>
            {save.isPending ? <Spinner size="sm" className="text-current" /> : null}
            {t('common:actions.save')}
          </Button>
        </>
      }
    >
      <div className="grid gap-4">
        <ol className="grid list-decimal gap-2 pl-5 text-sm marker:text-muted-foreground">
          <li>
            <Trans
              t={t}
              i18nKey="ytdlp.cookies.step1"
              components={{
                ext: (
                  <a
                    href={COOKIE_EXTENSION_URL}
                    target="_blank"
                    rel="noreferrer noopener"
                    className="inline-flex items-center gap-0.5 font-medium text-primary underline-offset-4 hover:underline"
                  />
                ),
                icon: <ExternalLink className="inline size-3" aria-hidden />,
              }}
            />
          </li>
          <li>{t(site === 'youtube' ? 'ytdlp.cookies.step2youtube' : 'ytdlp.cookies.step2', { site: siteName, domain })}</li>
          <li>{t('ytdlp.cookies.step3', { domain })}</li>
          <li>{t('ytdlp.cookies.step4', { domain })}</li>
        </ol>

        <Alert variant="destructive" className="py-2.5">
          <ShieldAlert />
          <AlertTitle>{t('ytdlp.warning.short')}</AlertTitle>
          <AlertDescription>{t('ytdlp.warning.local')}</AlertDescription>
        </Alert>

        <div className="grid gap-2">
          <div className="flex items-center justify-between gap-2">
            <label htmlFor="cookie-text" className="text-[13px] font-medium text-foreground/75">
              {t('ytdlp.cookies.paste')}
            </label>
            <Button type="button" variant="outline" size="sm" onClick={() => fileRef.current?.click()}>
              <FileText />
              {t('ytdlp.cookies.chooseFile')}
            </Button>
          </div>
          <Textarea
            id="cookie-text"
            value={text}
            onChange={(e) => setText(e.target.value)}
            rows={8}
            placeholder={'# Netscape HTTP Cookie File\n.' + domain + '\tTRUE\t/\tTRUE\t…'}
            className="max-h-64 min-h-36 font-mono text-xs break-all"
            // Credentials: no spell checking (some browsers send text to a cloud service),
            // no autofill, and password managers should leave it alone.
            spellCheck={false}
            autoComplete="off"
            autoCorrect="off"
            autoCapitalize="none"
            data-1p-ignore
            data-lpignore="true"
          />
          <input
            ref={fileRef}
            type="file"
            accept=".txt,text/plain"
            className="hidden"
            onChange={async (e) => {
              const file = e.target.files?.[0]
              e.target.value = ''
              if (!file) return
              if (file.size > MAX_FILE) {
                toast.error(t('ytdlp.cookies.tooLarge'))
                return
              }
              setText(await file.text())
            }}
          />
          <p className="text-xs text-muted-foreground">{t('ytdlp.cookies.keepsOnly', { domain })}</p>
        </div>
      </div>
    </ResponsiveDialog>
  )
}
