import { FolderOpen } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { useDefaultRenamePattern, useLibraries } from '@/features/admin/queries'

import { useDestination, useDestinationTarget } from '../lib/destination'
import { FolderPicker } from './folder-picker'

/**
 * Destination of added music (library, folder, organize by tags), shared by the Upload and
 * Online tabs. `children` are extra options shown under the destination.
 */
export function DestinationSection({ children }: { children?: ReactNode }) {
  const { t } = useTranslation('manage')
  const { libraries, limited } = useLibraries()
  const renamePattern = useDefaultRenamePattern()
  const dir = useDestination((s) => s.dir)
  const setDir = useDestination((s) => s.setDir)
  const setLibraryId = useDestination((s) => s.setLibraryId)
  const setOrganize = useDestination((s) => s.setOrganize)
  const target = useDestinationTarget()
  const [pickerOpen, setPickerOpen] = useState(false)

  return (
    <section className="grid gap-4 rounded-xl border p-4 sm:p-5">
      <h2 className="text-sm font-semibold">{t('upload.target')}</h2>
      {!limited && libraries.length > 1 ? (
        <div className="grid gap-1.5">
          <Label className="text-[13px] font-medium text-foreground/75">{t('filters.library')}</Label>
          <Select value={String(target.libraryId)} onValueChange={(v) => setLibraryId(Number(v))}>
            <SelectTrigger className="w-full sm:w-72">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {libraries.map((lib) => (
                <SelectItem key={lib.id} value={String(lib.id)}>
                  {lib.name || `#${lib.id}`}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      ) : null}
      <div className="grid gap-1.5">
        <Label htmlFor="destination-dir" className="text-[13px] font-medium text-foreground/75">
          {t('upload.folder')}
        </Label>
        <div className="flex gap-2">
          <Input
            id="destination-dir"
            value={dir}
            onChange={(e) => setDir(e.target.value)}
            placeholder={t('upload.folderPlaceholder')}
            aria-invalid={target.invalidDir || undefined}
            className="font-mono text-sm"
            autoComplete="off"
            spellCheck={false}
          />
          <Button variant="outline" onClick={() => setPickerOpen(true)} className="shrink-0 max-sm:size-11 max-sm:px-0">
            <FolderOpen />
            <span className="max-sm:sr-only">{t('upload.browse')}</span>
          </Button>
        </div>
        {target.invalidDir ? (
          <p className="text-xs text-destructive">{t('upload.invalidDir')}</p>
        ) : target.organize ? (
          <p className="text-xs text-muted-foreground">{t('upload.folderOrganizeHint')}</p>
        ) : null}
      </div>
      <Label className="flex items-start gap-3 font-normal">
        <Switch className="mt-0.5" checked={target.organize} onCheckedChange={setOrganize} />
        <span className="grid gap-1">
          <span className="text-sm font-medium">{t('upload.organize')}</span>
          <span className="text-xs text-muted-foreground">
            {t('upload.organizeHint')} <code className="font-mono break-all">{renamePattern}</code>
          </span>
        </span>
      </Label>
      {children}

      <FolderPicker
        open={pickerOpen}
        onOpenChange={setPickerOpen}
        libraryId={target.libraryId}
        libraryName={target.library?.name ?? ''}
        initialDir={target.dir}
        onSelect={setDir}
      />
    </section>
  )
}
