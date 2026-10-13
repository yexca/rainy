import { FolderOpen, FolderTree, Settings2 } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useLibraries } from '@/features/admin/queries'
import { useAuth } from '@/hooks/use-auth'

import { useDestination, useDestinationTarget } from '../lib/destination'
import { FolderPicker } from './folder-picker'

/**
 * Destination of added music (library and folder; whether files are organized by tags is a
 * server setting, shown here), shared by the Upload, Links and Online tabs. `children` are
 * extra options shown under the destination.
 */
export function DestinationSection({ children }: { children?: ReactNode }) {
  const { t } = useTranslation('manage')
  const { libraries, limited } = useLibraries()
  const { isAdmin } = useAuth()
  const dir = useDestination((s) => s.dir)
  const setDir = useDestination((s) => s.setDir)
  const setLibraryId = useDestination((s) => s.setLibraryId)
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
      <div className="flex flex-wrap items-center gap-3 rounded-lg bg-muted/50 px-3 py-2.5">
        <FolderTree className="size-[18px] shrink-0 text-muted-foreground" strokeWidth={1.75} aria-hidden />
        <p className="grid min-w-48 flex-1 gap-0.5">
          <span className="text-sm font-medium">{target.organize ? t('upload.organize') : t('upload.organizeOff')}</span>
          <span className="text-xs text-muted-foreground">
            {target.organize ? (
              <>
                {t('upload.organizeHint')} <code className="font-mono break-all">{target.renamePattern}</code>
              </>
            ) : (
              t('upload.organizeOffHint')
            )}
          </span>
        </p>
        {isAdmin ? (
          <Button asChild variant="outline" size="sm" className="max-sm:h-11 max-sm:w-full">
            <Link to="/admin/settings">
              <Settings2 />
              {t('upload.organizeSettings')}
            </Link>
          </Button>
        ) : null}
      </div>
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
