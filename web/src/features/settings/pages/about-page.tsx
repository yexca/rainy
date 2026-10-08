import { ArrowUpRight, CodeXml, Scale, ScrollText, type LucideIcon } from 'lucide-react'
import type { ReactNode } from 'react'
import { Trans, useTranslation } from 'react-i18next'

import { Logo } from '@/components/logo'
import { Page } from '@/components/page'
import { PageHeader } from '@/components/page-header'
import { useAuth } from '@/hooks/use-auth'
import { cn } from '@/lib/utils'

const REPOSITORY_URL = 'https://github.com/yexca/rainy'
const LICENSE_URL = `${REPOSITORY_URL}/blob/main/LICENSE`
const LICENSE_SPDX_ID = 'AGPL-3.0'
const AUTHOR = 'yexca'

/** The release page of a tagged build; development builds link to the release list. */
function releaseURL(version: string): string {
  if (!/^v?\d+\.\d+\.\d+$/.test(version)) return `${REPOSITORY_URL}/releases`
  return `${REPOSITORY_URL}/releases/tag/${version.startsWith('v') ? version : `v${version}`}`
}

const REFERENCE_PROJECTS = [
  { owner: 'navidrome', name: 'navidrome', url: 'https://github.com/navidrome/navidrome', description: 'aboutPage.navidromeReference' },
  {
    owner: 'lyswhut',
    name: 'lx-music-desktop',
    url: 'https://github.com/lyswhut/lx-music-desktop',
    description: 'aboutPage.lxMusicReference',
  },
  {
    owner: 'vinlxc',
    name: 'Music Tag',
    url: 'https://www.cnblogs.com/vinlxc/p/11347744.html',
    description: 'aboutPage.musicTagReference',
    closedSource: true,
  },
] as const

// Stored oldest first, displayed newest first. A null bound means an open-ended range.
const AI_MODEL_HISTORY = [{ from: null, to: null, models: ['Claude Opus 5.5', 'GPT-6.1-Sol'] }] as const

const TECHNOLOGY_GROUPS = [
  {
    key: 'frontend',
    items: ['React', 'TypeScript', 'Vite', 'Tailwind CSS', 'shadcn/ui', 'Radix UI', 'TanStack Query', 'Zustand', 'React Router', 'i18next', 'Motion', 'lucide-react'],
  },
  {
    key: 'backend',
    items: ['Go', 'chi', 'SQLite (modernc.org/sqlite)', 'sqlx', 'TagLib (go-taglib, WebAssembly)', 'goja', 'go-pinyin', 'golang.org/x/text'],
  },
  { key: 'standards', items: ['Subsonic API 1.16.1', 'OpenSubsonic', 'PWA', 'Media Session API'] },
  { key: 'runtime', items: ['FFmpeg', 'yt-dlp', 'Docker', 'GitHub Actions'] },
] as const

const focusRing = 'outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50'

/** `/about`: what Rainy is, how it was built, the projects it learned from, and its license. */
export default function AboutPage() {
  const { t } = useTranslation('settings')
  return (
    <Page>
      <PageHeader title={t('aboutPage.title')} back />
      <div className="grid max-w-5xl gap-x-12 gap-y-8 pb-4 lg:grid-cols-[15rem_minmax(0,1fr)] xl:grid-cols-[17rem_minmax(0,1fr)]">
        <Sleeve />
        <div className="min-w-0">
          <p className="max-w-2xl text-lg leading-8 font-medium tracking-tight sm:text-xl sm:leading-9">{t('aboutPage.intro')}</p>
          <div className="mt-8 grid gap-9">
            <Track number={1} title={t('aboutPage.overview')}>
              <div className="grid max-w-2xl gap-3 text-[15px] leading-relaxed text-muted-foreground md:text-sm">
                <p>{t('aboutPage.overviewOne')}</p>
                <p>{t('aboutPage.overviewTwo')}</p>
              </div>
            </Track>
            <Track number={2} title={t('aboutPage.builtWithAi')}>
              <p className="max-w-2xl text-[15px] leading-relaxed text-muted-foreground md:text-sm">{t('aboutPage.aiCredit')}</p>
              <ModelTimeline />
            </Track>
            <Track number={3} title={t('aboutPage.referenceProjects')}>
              <ReferenceList />
            </Track>
            <Track number={4} title={t('aboutPage.technologies')}>
              <TechnologyCredits />
            </Track>
          </div>
          <FinePrint />
        </div>
      </div>
    </Page>
  )
}

/** The record sleeve: artwork, name, version, and the outbound project links. */
function Sleeve() {
  const { t } = useTranslation('settings')
  const { version } = useAuth()
  return (
    <section
      aria-label={t('aboutPage.label')}
      className="grid grid-cols-[7rem_minmax(0,1fr)] items-center gap-5 sm:grid-cols-[9rem_minmax(0,1fr)] lg:sticky lg:top-[calc(var(--app-header-h)+4.5rem)] lg:grid-cols-1 lg:items-start lg:self-start"
    >
      <Artwork />
      <div className="min-w-0">
        <p className="text-xs font-medium tracking-[0.18em] text-muted-foreground uppercase">{AUTHOR}</p>
        <h2 className="mt-1 text-3xl leading-tight font-semibold tracking-tight sm:text-4xl">Rainy</h2>
        <span className="tnum mt-2.5 inline-flex rounded-md border bg-card px-2 py-0.5 font-mono text-xs text-muted-foreground select-all">
          {version || '—'}
        </span>
      </div>
      <ul className="col-span-2 grid grid-cols-3 gap-2 lg:col-span-1 lg:grid-cols-1 lg:gap-0 lg:divide-y lg:border-y">
        <SleeveLink href={REPOSITORY_URL} icon={CodeXml} label={t('aboutPage.sourceCode')} />
        <SleeveLink href={releaseURL(version)} icon={ScrollText} label={t('aboutPage.releaseNotes')} />
        <SleeveLink href={LICENSE_URL} icon={Scale} label={t('aboutPage.license')} meta={LICENSE_SPDX_ID} />
      </ul>
    </section>
  )
}

/** The app icon with a record that slides a little further out of the sleeve on hover. */
function Artwork() {
  return (
    <div className="group relative mr-[22%] lg:mr-[30%]">
      <div
        aria-hidden
        className="about-record absolute inset-y-[4%] left-[22%] aspect-square rounded-full transition-transform duration-500 ease-out group-hover:translate-x-[10%] motion-reduce:transition-none lg:left-[30%]"
      />
      <Logo size={240} className="relative aspect-square h-auto w-full rounded-[22%] shadow-md" />
    </div>
  )
}

function SleeveLink({ href, icon: Icon, label, meta }: { href: string; icon: LucideIcon; label: string; meta?: string }) {
  return (
    <li>
      <a
        href={href}
        target="_blank"
        rel="noreferrer"
        className={cn(
          'group flex min-h-11 flex-col items-center justify-center gap-1 rounded-xl border bg-card px-2 py-1.5 text-center text-xs font-medium transition-colors hover:bg-accent active:bg-accent',
          'lg:min-h-0 lg:flex-row lg:justify-start lg:gap-3 lg:rounded-none lg:border-0 lg:bg-transparent lg:px-1 lg:py-2.5 lg:text-left lg:text-sm lg:hover:bg-transparent',
          focusRing,
        )}
      >
        <Icon className="size-4 shrink-0 text-muted-foreground transition-colors group-hover:text-foreground" aria-hidden />
        <span className="lg:flex-1">{label}</span>
        {meta ? <span className="tnum hidden text-xs font-normal text-muted-foreground lg:inline">{meta}</span> : null}
        <ArrowUpRight
          aria-hidden
          className="hidden size-3.5 text-muted-foreground transition-transform group-hover:translate-x-px group-hover:-translate-y-px group-hover:text-foreground lg:block"
        />
      </a>
    </li>
  )
}

/** A liner-notes section, numbered like a track on the record. */
function Track({ number, title, children }: { number: number; title: string; children: ReactNode }) {
  return (
    <section aria-label={title}>
      <h3 className="flex items-baseline gap-3 border-b pb-2.5">
        <span aria-hidden className="tnum w-6 shrink-0 text-xs font-medium text-muted-foreground">
          {String(number).padStart(2, '0')}
        </span>
        <span className="text-base font-semibold">{title}</span>
      </h3>
      <div className="pt-4 sm:pl-9">{children}</div>
    </section>
  )
}

function ModelTimeline() {
  const { t } = useTranslation('settings')
  const entries = [...AI_MODEL_HISTORY].reverse()
  return (
    <div className="mt-5">
      <h4 className="text-xs font-medium text-muted-foreground">{t('aboutPage.modelHistoryTitle')}</h4>
      <ol className="mt-2 divide-y divide-dashed" aria-label={t('aboutPage.modelHistoryDescription')}>
        {entries.map((entry, index) => {
          const current = index === 0
          const range =
            entry.from && entry.from === entry.to
              ? entry.from
              : `${entry.from ?? t('aboutPage.firstRelease')} – ${entry.to ?? t('aboutPage.present')}`
          return (
            <li
              key={entry.from ?? 'start'}
              className="grid grid-cols-[1rem_minmax(0,1fr)] items-start gap-x-3 py-2 sm:grid-cols-[1rem_8.5rem_minmax(0,1fr)]"
            >
              <span className="flex h-5 items-center">
                {current ? <span className="size-1.5 rounded-full bg-primary" aria-hidden /> : null}
              </span>
              <p className={cn('tnum text-xs leading-5', current ? 'font-semibold' : 'text-muted-foreground')}>{range}</p>
              <p className={cn('col-start-2 text-sm leading-5 sm:col-start-3', !current && 'text-muted-foreground')}>
                {entry.models.join(' · ')}
              </p>
            </li>
          )
        })}
      </ol>
    </div>
  )
}

function ReferenceList() {
  const { t } = useTranslation('settings')
  return (
    <ul className="-mx-2 grid gap-1">
      {REFERENCE_PROJECTS.map((project) => (
        <li key={project.url} className="group relative rounded-xl px-2 py-2 transition-colors hover:bg-accent/60">
          <a
            href={project.url}
            target="_blank"
            rel="noreferrer"
            className="text-[15px] font-medium outline-none after:absolute after:inset-0 after:rounded-xl focus-visible:after:ring-[3px] focus-visible:after:ring-ring/50 md:text-sm"
          >
            <span className="font-normal text-muted-foreground">{project.owner}/</span>
            {project.name}
            <ArrowUpRight
              aria-hidden
              className="ml-1 inline size-3.5 align-[-0.125em] text-muted-foreground transition-colors group-hover:text-foreground"
            />
          </a>
          {'closedSource' in project ? (
            <span className="ml-2 inline-flex rounded-md border px-1.5 py-px align-[0.0625em] text-[11px] leading-4 text-muted-foreground">
              {t('aboutPage.closedSource')}
            </span>
          ) : null}
          <p className="mt-0.5 max-w-2xl text-[13px] leading-5 text-muted-foreground">{t(project.description)}</p>
        </li>
      ))}
    </ul>
  )
}

/** Technologies laid out like album credits: role on the left, names on the right. */
function TechnologyCredits() {
  const { t } = useTranslation('settings')
  return (
    <dl className="grid gap-y-4 sm:grid-cols-[7.5rem_minmax(0,1fr)] sm:gap-x-4 sm:gap-y-3">
      {TECHNOLOGY_GROUPS.map((group) => (
        <div key={group.key} className="min-w-0 sm:contents">
          <dt className="text-xs leading-6 font-medium tracking-[0.08em] text-muted-foreground uppercase">
            {t(`aboutPage.groups.${group.key}`)}
          </dt>
          <dd>
            <ul className="flex flex-wrap gap-x-1.5 text-sm leading-6 [&>li:not(:last-child)]:after:ml-1.5 [&>li:not(:last-child)]:after:text-muted-foreground [&>li:not(:last-child)]:after:content-['·']">
              {group.items.map((item) => (
                <li key={item}>{item}</li>
              ))}
            </ul>
          </dd>
        </div>
      ))}
    </dl>
  )
}

function FinePrint() {
  const { t } = useTranslation('settings')
  return (
    <footer className="mt-12 grid gap-2 border-t pt-4 text-xs leading-5 text-muted-foreground sm:pl-9">
      <p className="max-w-2xl font-medium text-foreground">{t('aboutPage.usageNotice')}</p>
      <p className="max-w-2xl">
        <span className="font-medium text-foreground">{t('aboutPage.copyright')}</span>{' '}
        <Trans
          t={t}
          i18nKey="aboutPage.licenseText"
          components={{
            license: (
              <a
                href={LICENSE_URL}
                target="_blank"
                rel="noreferrer"
                className={cn(
                  'rounded-sm font-medium text-foreground underline decoration-border underline-offset-4 transition-colors hover:decoration-foreground',
                  focusRing,
                )}
              />
            ),
          }}
        />
      </p>
    </footer>
  )
}
