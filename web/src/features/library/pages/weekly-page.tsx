import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import { MixView } from '../components/mix-view'
import { weekRangeLabel } from '../lib/daily'
import { weeklyMixQuery } from '../lib/queries'

/** `/weekly`: this week's weekly mix (§9.5a), the same songs from Monday to Sunday. */
export default function WeeklyPage() {
  const { t } = useTranslation('library')
  const query = useQuery(weeklyMixQuery())
  return (
    <MixView
      query={query}
      title={t('weekly.title')}
      period={query.data ? weekRangeLabel(query.data.week) : undefined}
      description={t('weekly.description')}
      emptyTitle={t('weekly.emptyTitle')}
      emptyDescription={t('weekly.emptyDescription')}
    />
  )
}
