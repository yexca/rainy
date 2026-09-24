import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ChevronRight, KeyRound, MoreHorizontal, Pencil, Trash2, UserPlus, Users } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { Page } from '@/components/page'
import { PageHeader } from '@/components/page-header'
import { PageLoader } from '@/components/spinner'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { ConfirmDialog } from '@/features/manage/components/confirm-dialog'
import { useCurrentUser } from '@/hooks/use-auth'
import { useIsMobile } from '@/hooks/use-media-query'
import { api } from '@/lib/api/endpoints'
import type { User } from '@/lib/api/types'
import { errorMessage } from '@/lib/errors'
import { formatRelative } from '@/lib/format'
import { initials } from '@/lib/utils'

import { PasswordDialog } from '../components/password-dialog'
import { UserDialog } from '../components/user-dialog'
import { adminKeys } from '../queries'

export default function UsersPage() {
  const { t } = useTranslation('admin')
  const isMobile = useIsMobile()
  const queryClient = useQueryClient()
  const me = useCurrentUser()
  const [editing, setEditing] = useState<{ user: User | null } | null>(null)
  const [resetting, setResetting] = useState<User | null>(null)
  const [deleting, setDeleting] = useState<User | null>(null)

  const users = useQuery({ queryKey: adminKeys.users, queryFn: ({ signal }) => api.admin.users.list({ signal }) })
  const list = [...(users.data ?? [])].sort((a, b) => Number(b.isAdmin) - Number(a.isAdmin) || a.username.localeCompare(b.username))

  const remove = async () => {
    if (!deleting) return
    try {
      await api.admin.users.delete(deleting.id)
      toast.success(t('users.deleted', { name: deleting.username }))
      void queryClient.invalidateQueries({ queryKey: adminKeys.users })
    } catch (error) {
      toast.error(errorMessage(error, t))
      throw error
    }
  }

  const actions = (user: User) => ({
    edit: () => setEditing({ user }),
    reset: () => setResetting(user),
    remove: () => setDeleting(user),
    isSelf: user.id === me?.id,
  })

  return (
    <Page>
      <PageHeader
        title={t('users.title')}
        subtitle={users.data ? t('users.count', { count: list.length }) : t('users.subtitle')}
        back={isMobile ? '/manage' : undefined}
        actions={
          isMobile ? null : (
            <Button onClick={() => setEditing({ user: null })}>
              <UserPlus />
              {t('users.add')}
            </Button>
          )
        }
        navActions={
          isMobile ? (
            <Button variant="ghost" size="icon" className="size-11 text-primary" onClick={() => setEditing({ user: null })} aria-label={t('users.add')}>
              <UserPlus className="size-5" />
            </Button>
          ) : null
        }
      />

      {users.isPending ? (
        <PageLoader />
      ) : users.isError ? (
        <ErrorState error={users.error} onRetry={() => void users.refetch()} retrying={users.isFetching} />
      ) : list.length === 0 ? (
        <EmptyState icon={Users} title={t('users.empty')} />
      ) : isMobile ? (
        <ul className="bleed-x">
          {list.map((user) => (
            <li key={user.id} className="hairline-inset [--hairline-inset:calc(var(--page-px)+3.5rem)]">
              <button type="button" onClick={() => setEditing({ user })} className="page-x flex min-h-16 w-full items-center gap-3 py-2 text-left active:bg-accent/60">
                <UserAvatar user={user} />
                <span className="grid min-w-0 flex-1 gap-1">
                  <span className="flex items-center gap-2">
                    <span className="truncate text-[15px] font-medium">{user.displayName || user.username}</span>
                    {user.id === me?.id ? <Badge variant="secondary">{t('users.you')}</Badge> : null}
                  </span>
                  <RoleBadges user={user} />
                </span>
                <ChevronRight className="size-4 text-muted-foreground/60" />
              </button>
            </li>
          ))}
        </ul>
      ) : (
        <div className="overflow-hidden rounded-xl border">
          <table className="w-full text-sm">
            <thead className="bg-muted/40 text-left text-xs text-muted-foreground">
              <tr>
                <th className="px-4 py-2.5 font-medium">{t('users.fields.user')}</th>
                <th className="px-4 py-2.5 font-medium max-lg:hidden">{t('users.fields.email')}</th>
                <th className="px-4 py-2.5 font-medium">{t('users.fields.roles')}</th>
                <th className="px-4 py-2.5 font-medium">{t('users.fields.lastSeen')}</th>
                <th className="w-12 px-2 py-2.5" />
              </tr>
            </thead>
            <tbody className="divide-y">
              {list.map((user) => {
                const a = actions(user)
                return (
                  <tr key={user.id} className="hover:bg-accent/30">
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-3">
                        <UserAvatar user={user} />
                        <div className="min-w-0">
                          <p className="flex items-center gap-2 font-medium">
                            <span className="truncate">{user.displayName || user.username}</span>
                            {a.isSelf ? <Badge variant="secondary">{t('users.you')}</Badge> : null}
                          </p>
                          <p className="truncate text-xs text-muted-foreground">@{user.username}</p>
                        </div>
                      </div>
                    </td>
                    <td className="px-4 py-3 text-muted-foreground max-lg:hidden">{user.email || '—'}</td>
                    <td className="px-4 py-3">
                      <RoleBadges user={user} />
                    </td>
                    <td className="tnum px-4 py-3 text-muted-foreground">{formatRelative(user.lastSeenAt || user.lastLoginAt)}</td>
                    <td className="px-2 py-3">
                      <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                          <Button variant="ghost" size="icon-sm" aria-label={t('common:actions.more')}>
                            <MoreHorizontal />
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end" className="min-w-48 rounded-xl">
                          <DropdownMenuItem onSelect={a.edit}>
                            <Pencil />
                            {t('common:actions.edit')}
                          </DropdownMenuItem>
                          <DropdownMenuItem onSelect={a.reset}>
                            <KeyRound />
                            {t('users.resetTitle')}
                          </DropdownMenuItem>
                          <DropdownMenuSeparator />
                          <DropdownMenuItem variant="destructive" disabled={a.isSelf} onSelect={a.remove}>
                            <Trash2 />
                            {t('common:actions.delete')}
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}

      <UserDialog
        open={editing !== null}
        onOpenChange={(open) => (open ? undefined : setEditing(null))}
        user={editing?.user ?? null}
        currentUserId={me?.id ?? ''}
        onResetPassword={(u) => {
          setEditing(null)
          setResetting(u)
        }}
        onDelete={(u) => {
          setEditing(null)
          setDeleting(u)
        }}
      />
      <PasswordDialog user={resetting} onOpenChange={(open) => (open ? undefined : setResetting(null))} />
      <ConfirmDialog
        open={deleting !== null}
        onOpenChange={(open) => (open ? undefined : setDeleting(null))}
        title={t('users.deleteTitle', { name: deleting?.username ?? '' })}
        description={t('users.deleteDescription')}
        confirmLabel={t('common:actions.delete')}
        destructive
        onConfirm={remove}
      />
    </Page>
  )
}

function UserAvatar({ user }: { user: User }) {
  return (
    <Avatar className="size-10">
      <AvatarFallback className="bg-primary/12 text-sm font-semibold text-primary">{initials(user.displayName || user.username)}</AvatarFallback>
    </Avatar>
  )
}

function RoleBadges({ user }: { user: User }) {
  const { t } = useTranslation('admin')
  const roles: string[] = []
  if (user.isAdmin) roles.push(t('roles.admin'))
  else {
    if (user.canManage) roles.push(t('roles.manager'))
    if (user.canDownload) roles.push(t('roles.download'))
  }
  if (roles.length === 0) roles.push(t('roles.listener'))
  return (
    <span className="flex flex-wrap gap-1">
      {roles.map((r, i) => (
        <Badge key={r} variant={user.isAdmin && i === 0 ? 'default' : 'secondary'} className="font-normal">
          {r}
        </Badge>
      ))}
    </span>
  )
}
