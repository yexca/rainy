import { Check, ChevronDown, Info, Languages, LogOut, Monitor, Moon, Settings, Sun } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate } from 'react-router'

import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useAuth, useLogout } from '@/hooks/use-auth'
import { useTheme } from '@/hooks/use-theme'
import type { User } from '@/lib/api/types'
import { LANGUAGES, currentLanguage, setLanguage } from '@/lib/i18n'
import { isTheme } from '@/lib/theme'
import { cn, initials } from '@/lib/utils'

function displayName(user: User): string {
  return user.displayName.trim() || user.username
}

function roleKey(user: User): string {
  if (user.isAdmin) return 'user.admin'
  if (user.canManage) return 'user.manager'
  return 'user.listener'
}

export function UserAvatar({ user, className }: { user: User; className?: string }) {
  return (
    <Avatar className={cn('size-8 rounded-full', className)}>
      <AvatarFallback className="bg-primary/15 text-xs font-semibold text-primary">
        {initials(displayName(user))}
      </AvatarFallback>
    </Avatar>
  )
}

function UserMenuContent({ user }: { user: User }) {
  const { t } = useTranslation()
  const { theme, setTheme } = useTheme()
  const logout = useLogout()
  const navigate = useNavigate()
  const language = currentLanguage()
  const ThemeIcon = theme === 'dark' ? Moon : theme === 'light' ? Sun : Monitor

  const onLogout = () => {
    logout.mutate(undefined, {
      onSettled: () => navigate('/login', { replace: true }),
    })
  }

  return (
    <DropdownMenuContent side="bottom" align="end" sideOffset={8} className="min-w-60 rounded-xl">
      <DropdownMenuLabel className="flex items-center gap-3 py-2 font-normal">
        <UserAvatar user={user} />
        <div className="grid min-w-0 flex-1 leading-tight">
          <span className="truncate text-sm font-semibold">{displayName(user)}</span>
          <span className="truncate text-xs text-muted-foreground">
            @{user.username} · {t(roleKey(user))}
          </span>
        </div>
      </DropdownMenuLabel>
      <DropdownMenuSeparator />
      <DropdownMenuGroup>
        <DropdownMenuItem asChild>
          <Link to="/settings">
            <Settings />
            {t('nav.settings')}
          </Link>
        </DropdownMenuItem>
        <DropdownMenuSub>
          <DropdownMenuSubTrigger>
            <ThemeIcon className="size-4 text-muted-foreground" />
            {t('theme.label')}
          </DropdownMenuSubTrigger>
          <DropdownMenuSubContent className="rounded-xl">
            <DropdownMenuRadioGroup value={theme} onValueChange={(value) => isTheme(value) && setTheme(value)}>
              <DropdownMenuRadioItem value="light">{t('theme.light')}</DropdownMenuRadioItem>
              <DropdownMenuRadioItem value="dark">{t('theme.dark')}</DropdownMenuRadioItem>
              <DropdownMenuRadioItem value="system">{t('theme.system')}</DropdownMenuRadioItem>
            </DropdownMenuRadioGroup>
          </DropdownMenuSubContent>
        </DropdownMenuSub>
        <DropdownMenuSub>
          <DropdownMenuSubTrigger>
            <Languages className="size-4 text-muted-foreground" />
            {t('language.label')}
          </DropdownMenuSubTrigger>
          <DropdownMenuSubContent className="rounded-xl">
            {LANGUAGES.map((lang) => (
              <DropdownMenuItem
                key={lang.code}
                lang={lang.htmlLang}
                onSelect={() => void setLanguage(lang.code)}
              >
                <span className="flex-1">{lang.label}</span>
                {language === lang.code ? <Check className="text-primary" /> : null}
              </DropdownMenuItem>
            ))}
          </DropdownMenuSubContent>
        </DropdownMenuSub>
        <DropdownMenuItem asChild>
          <Link to="/about">
            <Info />
            {t('nav.about')}
          </Link>
        </DropdownMenuItem>
      </DropdownMenuGroup>
      <DropdownMenuSeparator />
      <DropdownMenuItem onSelect={onLogout} disabled={logout.isPending}>
        <LogOut />
        {t('actions.logout')}
      </DropdownMenuItem>
    </DropdownMenuContent>
  )
}

export interface UserMenuProps {
  /** `header`: avatar and name for the app header (tablet / desktop). `avatar`: round button (phone nav bars). */
  variant?: 'header' | 'avatar'
  className?: string
}

/** Account menu: settings, theme, language and logout. */
export function UserMenu({ variant = 'avatar', className }: UserMenuProps) {
  const { t } = useTranslation()
  const { user } = useAuth()
  if (!user) return null

  if (variant === 'header') {
    return (
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button
            type="button"
            aria-label={t('user.menu')}
            className={cn(
              'flex h-9 min-w-0 items-center gap-2 rounded-full p-0.5 text-left outline-none transition-colors hover:bg-sidebar-accent focus-visible:ring-2 focus-visible:ring-sidebar-ring data-[state=open]:bg-sidebar-accent lg:pr-2.5',
              className,
            )}
          >
            <UserAvatar user={user} />
            <span className="hidden max-w-40 truncate text-sm font-medium lg:block">{displayName(user)}</span>
            <ChevronDown className="hidden size-4 shrink-0 text-muted-foreground lg:block" aria-hidden />
          </button>
        </DropdownMenuTrigger>
        <UserMenuContent user={user} />
      </DropdownMenu>
    )
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" aria-label={t('user.menu')} className={cn('size-11 rounded-full', className)}>
          <UserAvatar user={user} />
        </Button>
      </DropdownMenuTrigger>
      <UserMenuContent user={user} />
    </DropdownMenu>
  )
}
