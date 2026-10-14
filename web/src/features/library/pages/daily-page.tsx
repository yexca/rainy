import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import { MixView } from '../components/mix-view'
import { dailyDateLabel } from '../lib/daily'
import { dailyMixQuery } from '../lib/queries'

/** `/daily`: today's daily mix (§9.5a), the same songs all day. */
export default function DailyPage() {
  const { t } = useTranslation('library')
  const query = useQuery(dailyMixQuery())
  return (
    <MixView
      query={query}
      title={t('daily.title')}
      period={query.data ? dailyDateLabel(query.data.date) : undefined}
      description={t('daily.description')}
      emptyTitle={t('daily.emptyTitle')}
      emptyDescription={t('daily.emptyDescription')}
    />
  )
}
