import { useMutation, useQueryClient } from '@tanstack/react-query'
import { FileCode, Link2, ShieldAlert } from 'lucide-react'
import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Spinner } from '@/components/spinner'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { ResponsiveDialog } from '@/features/manage/components/responsive-dialog'
import { api } from '@/lib/api/endpoints'
import type { LxSource } from '@/lib/api/types'
import { errorMessage } from '@/lib/errors'

import { adminKeys } from '../queries'

/** lx-music accepts scripts up to 9,000,000 characters; the server checks bytes. */
const MAX_SCRIPT = 9_000_000

export interface SourceImportDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Importing from a link needs the feature on (the server downloads the script). */
  enabled: boolean
}

/** Imports an lx-music custom source script from a .js file or a link. */
export function SourceImportDialog({ open, onOpenChange, enabled }: SourceImportDialogProps) {
  const { t } = useTranslation('admin')
  const queryClient = useQueryClient()
  const [mode, setMode] = useState<'file' | 'url'>('file')
  const [file, setFile] = useState<File | null>(null)
  const [url, setUrl] = useState('')
  const fileRef = useRef<HTMLInputElement>(null)

  const close = (next: boolean) => {
    if (!next) {
      setFile(null)
      setUrl('')
    }
    onOpenChange(next)
  }

  const save = useMutation({
    mutationFn: async (): Promise<LxSource> => {
      if (mode === 'url') return api.admin.sources.import({ url: url.trim() })
      if (!file) throw new Error(t('sources.import.chooseFile'))
      if (file.size > MAX_SCRIPT) throw new Error(t('sources.import.tooLarge'))
      return api.admin.sources.import({ script: await file.text() })
    },
    onSuccess: (source) => {
      void queryClient.invalidateQueries({ queryKey: adminKeys.sources })
      void queryClient.invalidateQueries({ queryKey: ['manage', 'online'] })
      if (source.status === 'error') toast.warning(t('sources.import.importedWithError', { name: source.name }), { description: source.error })
      else toast.success(t('sources.import.imported', { name: source.name }))
      close(false)
    },
    onError: (error) => toast.error(errorMessage(error, t)),
  })

  const canSave = !save.isPending && (mode === 'file' ? file !== null : enabled && /^https?:\/\/\S+$/i.test(url.trim()))

  return (
    <ResponsiveDialog
      open={open}
      onOpenChange={close}
      dismissible={!save.isPending}
      title={t('sources.import.title')}
      description={t('sources.import.description')}
      footer={
        <>
          <Button variant="ghost" onClick={() => close(false)} disabled={save.isPending} className="max-sm:h-11">
            {t('common:actions.cancel')}
          </Button>
          <Button onClick={() => save.mutate()} disabled={!canSave} className="max-sm:h-11">
            {save.isPending ? <Spinner size="sm" className="text-current" /> : null}
            {save.isPending ? t('sources.import.importing') : t('sources.import.submit')}
          </Button>
        </>
      }
    >
      <div className="grid gap-4">
        <Alert>
          <ShieldAlert />
          <AlertTitle>{t('sources.import.warningTitle')}</AlertTitle>
          <AlertDescription>{t('sources.import.warning')}</AlertDescription>
        </Alert>
        <Tabs value={mode} onValueChange={(v) => setMode(v === 'url' ? 'url' : 'file')}>
          <TabsList className="w-full">
            <TabsTrigger value="file">
              <FileCode />
              {t('sources.import.fromFile')}
            </TabsTrigger>
            <TabsTrigger value="url">
              <Link2 />
              {t('sources.import.fromUrl')}
            </TabsTrigger>
          </TabsList>
        </Tabs>
        {mode === 'file' ? (
          <div className="grid gap-2">
            <input
              ref={fileRef}
              type="file"
              accept=".js,text/javascript,application/javascript"
              className="hidden"
              onChange={(e) => {
                setFile(e.target.files?.[0] ?? null)
                e.target.value = ''
              }}
            />
            <Button variant="outline" onClick={() => fileRef.current?.click()} className="justify-start max-sm:h-11">
              <FileCode />
              <span className="truncate">{file ? file.name : t('sources.import.chooseFile')}</span>
            </Button>
            <p className="text-xs text-muted-foreground">{t('sources.import.fileHint')}</p>
          </div>
        ) : (
          <div className="grid gap-2">
            <Label htmlFor="source-url" className="text-[13px] font-medium text-foreground/75">
              {t('sources.import.url')}
            </Label>
            <Input
              id="source-url"
              type="url"
              inputMode="url"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              placeholder="https://example.com/source.js"
              autoComplete="off"
              autoCapitalize="none"
              spellCheck={false}
              className="font-mono text-sm max-sm:h-11"
              disabled={!enabled}
            />
            <p className="text-xs text-muted-foreground">{enabled ? t('sources.import.urlHint') : t('sources.import.urlNeedsEnable')}</p>
          </div>
        )}
      </div>
    </ResponsiveDialog>
  )
}
