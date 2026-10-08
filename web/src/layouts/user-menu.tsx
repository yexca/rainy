import { ChevronDown, Info, LogOut, Monitor, Moon, Settings, Sun } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate } from 'react-router'

import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Badge } from '@/components/ui/badge'
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
import { LANGUAGES, currentLanguage, setLanguage, type Language } from '@/lib/i18n'
import { isTheme } from '@/lib/theme'
import { cn, initials } from '@/lib/utils'

function isLanguage(value: string): value is Language {
  return LANGUAGES.some((lang) => lang.code === value)
}

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

/** `showAppearance`: phones have no header tray, so theme and language live in this menu there. */
function UserMenuContent({ user, showAppearance }: { user: User; showAppearance: boolean }) {
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
    <DropdownMenuContent side="bottom" align="end" sideOffset={8} className="w-64 rounded-xl">
      <DropdownMenuLabel className="flex items-center gap-3 px-2 py-2.5 font-normal">
        <UserAvatar user={user} className="size-11 [&>span]:text-base" />
        <div className="grid min-w-0 flex-1 gap-1 leading-tight">
          <span className="truncate text-sm font-semibold">{displayName(user)}</span>
          <span className="truncate text-xs text-muted-foreground">@{user.username}</span>
          <Badge variant="outline" className="px-1.5 py-0 text-[11px] font-medium">
            {t(roleKey(user))}
          </Badge>
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
        {showAppearance ? (
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
              <DropdownMenuSeparator />
              <DropdownMenuLabel className="text-xs font-medium text-muted-foreground">{t('language.label')}</DropdownMenuLabel>
              <DropdownMenuRadioGroup value={language} onValueChange={(value) => isLanguage(value) && void setLanguage(value)}>
                {LANGUAGES.map((lang) => (
                  <DropdownMenuRadioItem key={lang.code} value={lang.code} lang={lang.htmlLang}>
                    {lang.label}
                  </DropdownMenuRadioItem>
                ))}
              </DropdownMenuRadioGroup>
            </DropdownMenuSubContent>
          </DropdownMenuSub>
        ) : null}
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
  /**
   * `header`: avatar, name and role for the app header (tablet / desktop; theme and language live
   * in the header tray). `avatar`: round button (phone nav bars).
   */
  variant?: 'header' | 'avatar'
  className?: string
}

/** Account menu: profile, settings and logout (plus theme and language on phones). */
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
              'flex h-10 min-w-0 items-center gap-2 rounded-full p-1 text-left outline-none transition-colors hover:bg-sidebar-accent focus-visible:ring-2 focus-visible:ring-sidebar-ring data-[state=open]:bg-sidebar-accent lg:pr-2.5',
              className,
            )}
          >
            <UserAvatar user={user} />
            <span className="hidden min-w-0 lg:block">
              <span className="block max-w-36 truncate text-xs leading-4 font-medium">{displayName(user)}</span>
              <span className="block max-w-36 truncate text-[11px] leading-3.5 text-muted-foreground">{t(roleKey(user))}</span>
            </span>
            <ChevronDown className="hidden size-3.5 shrink-0 text-muted-foreground lg:block" aria-hidden />
          </button>
        </DropdownMenuTrigger>
        <UserMenuContent user={user} showAppearance={false} />
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
      <UserMenuContent user={user} showAppearance />
    </DropdownMenu>
  )
}
