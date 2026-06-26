import * as React from 'react'
import { useTranslation } from 'react-i18next'
import { Button, Chip, Link, Separator, Switch, toast } from '@heroui/react'
import type { Selection } from '@heroui/react'
import { EmptyState } from '@heroui-pro/react'
import { api } from '../../api/client'
import type { EventDefinition, EventClaimRecord, EventsConfig } from '../../api/client'
import { usePermissions } from '../../hooks/usePermissions'
import { ActionBar, ConfirmDialog, DataTable, Icon, PageHeader, Panel, SectionLabel, type Column } from '../../dune-ui'
import { EventEditorModal } from './modals/EventEditorModal'
import type { ListKey, ClaimKey } from './types'

export const EventsTab: React.FC = () => {
  const { t } = useTranslation()
  const { can } = usePermissions()
  const [events, setEvents] = React.useState<EventDefinition[]>([])
  const [loading, setLoading] = React.useState(false)
  const [selectedEvent, setSelectedEvent] = React.useState<EventDefinition | null>(null)
  const [claims, setClaims] = React.useState<EventClaimRecord[]>([])
  const [claimsLoading, setClaimsLoading] = React.useState(false)
  const [editorOpen, setEditorOpen] = React.useState(false)
  const [editingEvent, setEditingEvent] = React.useState<EventDefinition | null>(null)
  const [selectedKeys, setSelectedKeys] = React.useState<Selection>(new Set())
  const [cfg, setCfg] = React.useState<EventsConfig>({ events_enabled: false })
  const [cfgSaving, setCfgSaving] = React.useState(false)
  const [deleteTarget, setDeleteTarget] = React.useState<EventDefinition | null>(null)
  const [resetTarget, setResetTarget] = React.useState<EventDefinition | null>(null)

  const selectionCount = selectedKeys === 'all' ? events.length : (selectedKeys as Set<string>).size

  const handleBulkDelete = () => {
    const ids = selectedKeys === 'all'
      ? events.map((e) => e.id)
      : [...(selectedKeys as Set<string>)].map(Number)
    setSelectedKeys(new Set())
    Promise.all(ids.map((id) => api.events.delete(id)))
      .then(() => {
        if (selectedEvent && ids.includes(selectedEvent.id)) setSelectedEvent(null)
        loadEvents()
      })
      .catch((e: unknown) => {
        toast.danger(t('events.deleteFailed', { message: e instanceof Error ? e.message : String(e) }))
        loadEvents()
      })
  }

  const LIST_COLUMNS: Column<ListKey>[] = [
    { key: 'name', label: t('events.columns.name'), minWidth: 200 },
    { key: 'type', label: t('events.columns.type'), width: 120 },
    { key: 'enabled', label: t('events.columns.enabled'), width: 90, sortable: false },
    { key: 'version', label: t('events.columns.claims'), width: 80 },
    { key: 'actions', label: '', width: 120, sortable: false },
  ]

  const CLAIM_COLUMNS: Column<ClaimKey>[] = [
    { key: 'account_id', label: t('events.claims.accountId'), width: 110 },
    { key: 'version', label: t('events.claims.version'), width: 70 },
    { key: 'status', label: t('events.claims.status'), width: 110 },
    { key: 'attempts', label: t('events.claims.attempts'), width: 80 },
    { key: 'next_attempt_at', label: t('events.claims.nextAttempt'), minWidth: 160 },
    { key: 'claimed_at', label: t('events.claims.claimedAt'), minWidth: 160 },
    { key: 'last_error', label: t('events.claims.lastError'), minWidth: 200 },
    { key: 'actions', label: t('events.claims.actions'), width: 110, sortable: false },
  ]

  const claimStatusColor = (status: string): 'success' | 'warning' | 'danger' => {
    if (status === 'granted') return 'success'
    if (status === 'pending') return 'warning'
    return 'danger' // exhausted or legacy "failed"
  }

  const claimStatusLabel = (status: string): string => {
    const labels = t('events.claims.statusLabels', { returnObjects: true }) as Record<string, string>
    return labels[status] ?? status
  }

  // pending, exhausted, and legacy "failed" claims can be manually re-granted.
  const canGrantClaim = (status: string): boolean =>
    status === 'pending' || status === 'exhausted' || status === 'failed'

  const handleGrantClaim = (claim: EventClaimRecord) => {
    api.events
      .grantClaim(claim.event_id, claim.account_id)
      .then(() => {
        toast.success(t('events.grantSuccess'))
        if (selectedEvent) loadStatus(selectedEvent)
      })
      .catch((e: unknown) => {
        toast.danger(t('events.grantFailed', { message: e instanceof Error ? e.message : String(e) }))
      })
  }

  const loadEvents = (): void => {
    Promise.resolve()
      .then(() => setLoading(true))
      .then(() => api.events.list())
      .then(setEvents)
      .catch((e: unknown) => {
        toast.danger(t('events.failedToLoad', { message: e instanceof Error ? e.message : String(e) }))
      })
      .finally(() => setLoading(false))
  }

  const loadStatus = (ev: EventDefinition): void => {
    setSelectedEvent(ev)
    setClaimsLoading(true)
    api.events
      .status(ev.id)
      .then((s) => setClaims(s.claims))
      .catch((e: unknown) => {
        toast.danger(t('events.failedToLoadStatus', { message: e instanceof Error ? e.message : String(e) }))
      })
      .finally(() => setClaimsLoading(false))
  }

  const loadConfig = (): void => {
    api.events.config()
      .then(setCfg)
      .catch(() => { /* silent — config non-critical */ })
  }

  const toggleEnabled = (enabled: boolean) => {
    const prev = cfg
    setCfg((p) => ({ ...p, events_enabled: enabled }))
    setCfgSaving(true)
    api.events.saveConfig({ ...cfg, events_enabled: enabled })
      .then(setCfg)
      .catch((e: unknown) => {
        setCfg(prev)
        toast.danger(t('events.config.saveFailed', { message: e instanceof Error ? e.message : String(e) }))
      })
      .finally(() => setCfgSaving(false))
  }

  React.useEffect(() => {
    loadEvents()
    loadConfig()
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  const handleToggleEnabled = (ev: EventDefinition) => {
    api.events
      .setEnabled(ev.id, !ev.enabled)
      .then(loadEvents)
      .catch((e: unknown) => {
        toast.danger(t('events.toggleFailed', { message: e instanceof Error ? e.message : String(e) }))
      })
  }

  const handleDelete = (ev: EventDefinition) => {
    setDeleteTarget(ev)
  }

  const confirmDelete = () => {
    if (!deleteTarget) return
    const ev = deleteTarget
    setDeleteTarget(null)
    api.events
      .delete(ev.id)
      .then(() => {
        if (selectedEvent?.id === ev.id) setSelectedEvent(null)
        loadEvents()
      })
      .catch((e: unknown) => {
        toast.danger(t('events.deleteFailed', { message: e instanceof Error ? e.message : String(e) }))
      })
  }

  const handleReset = (ev: EventDefinition) => {
    setResetTarget(ev)
  }

  const confirmReset = () => {
    if (!resetTarget) return
    const ev = resetTarget
    setResetTarget(null)
    api.events
      .reset(ev.id)
      .then(() => {
        toast.success(t('events.resetSuccess'))
        if (selectedEvent?.id === ev.id) loadStatus(ev)
      })
      .catch((e: unknown) => {
        toast.danger(t('events.resetFailed', { message: e instanceof Error ? e.message : String(e) }))
      })
  }

  const openCreate = () => {
    setEditingEvent(null)
    setEditorOpen(true)
  }

  const openEdit = (ev: EventDefinition) => {
    setEditingEvent(ev)
    setEditorOpen(true)
  }

  const typeChipColor = (type: EventDefinition['type']): 'warning' | 'accent' =>
    type === 'zone_race' ? 'warning' : 'accent'

  return (
    <React.Fragment>
      <div className="flex flex-col h-full gap-3 min-h-0">
        <PageHeader title={t('events.title', { count: events.length })} subtitle={t('events.subtitle')}>
          {can('events:manage') && (
            <Switch
              isSelected={cfg.events_enabled ?? false}
              onChange={toggleEnabled}
              isDisabled={cfgSaving}
              size="sm"
            >
              <Switch.Content>
                <Switch.Control><Switch.Thumb /></Switch.Control>
                {t('events.config.enabled')}
              </Switch.Content>
            </Switch>
          )}
          <Button size="sm" variant="ghost" onPress={loadEvents} isDisabled={loading}>
            <Icon name="refresh-cw" />
            {' '}
            {t('common.refresh')}
          </Button>
          {can('events:manage') && (
            <Button size="sm" variant="primary" onPress={openCreate}>
              <Icon name="plus" />
              {' '}
              {t('events.create')}
            </Button>
          )}
        </PageHeader>

        <DataTable<EventDefinition, ListKey>
          selectionMode="multiple"
          selectedKeys={selectedKeys}
          onSelectionChange={setSelectedKeys}
          aria-label={t('events.ariaLabel')}
          className="min-h-0 flex-1"
          columns={LIST_COLUMNS}
          rows={events}
          loading={loading}
          rowId={(e) => String(e.id)}
          initialSort={{ column: 'name', direction: 'ascending' }}
          sortValue={(e, k) => {
            if (k === 'enabled') return e.enabled ? 1 : 0
            if (k === 'actions' || k === 'version') return ''
            return (e as unknown as Record<string, string | number>)[k] ?? ''
          }}
          emptyState={(
            <EmptyState size="sm">
              <EmptyState.Header>
                <EmptyState.Title>{t('events.noEvents')}</EmptyState.Title>
              </EmptyState.Header>
            </EmptyState>
          )}
          renderCell={(ev, key) => {
            switch (key) {
              case 'name':
                return (
                  <Link
                    className="text-left text-accent"
                    onPress={() => loadStatus(ev)}
                  >
                    {ev.name}
                  </Link>
                )
              case 'type':
                return (
                  <Chip size="sm" variant="soft" color={typeChipColor(ev.type)}>
                    {ev.type === 'zone_race' ? t('events.types.zoneRace') : t('events.types.milestone')}
                  </Chip>
                )
              case 'enabled':
                return can('events:manage')
                  ? (
                      <Switch
                        size="sm"
                        isSelected={ev.enabled}
                        onChange={() => handleToggleEnabled(ev)}
                        aria-label={t('events.toggleEnabled')}
                      >
                        <Switch.Content>
                          <Switch.Control><Switch.Thumb /></Switch.Control>
                        </Switch.Content>
                      </Switch>
                    )
                  : (
                      <span className="text-muted text-xs">
                        {ev.enabled ? <Icon name="check" /> : '—'}
                      </span>
                    )
              case 'version':
                return <span className="text-muted font-mono text-xs">{ev.version}</span>
              case 'actions':
                return can('events:manage')
                  ? (
                      <div className="flex gap-1">
                        <Button size="sm" variant="ghost" onPress={() => openEdit(ev)} aria-label={t('common.edit') as string}>
                          <Icon name="pencil" />
                        </Button>
                        <Button size="sm" variant="danger-soft" onPress={() => handleDelete(ev)} aria-label={t('common.delete') as string}>
                          <Icon name="trash-2" />
                        </Button>
                      </div>
                    )
                  : null
            }
          }}
        />

        {selectedEvent && (
          <Panel>
            <div className="flex items-center justify-between mb-3">
              <SectionLabel>
                {t('events.status.title', { name: selectedEvent.name })}
              </SectionLabel>
              <div className="flex gap-2">
                <Button size="sm" variant="ghost" onPress={() => loadStatus(selectedEvent)} isDisabled={claimsLoading}>
                  <Icon name="refresh-cw" />
                </Button>
                {can('events:manage') && (
                  <Button size="sm" variant="outline" onPress={() => handleReset(selectedEvent)}>
                    {t('events.status.reset')}
                  </Button>
                )}
                <Button size="sm" variant="ghost" onPress={() => setSelectedEvent(null)}>
                  <Icon name="x" />
                </Button>
              </div>
            </div>
            <DataTable<EventClaimRecord, ClaimKey>
              aria-label={t('events.status.claimsLabel')}
              className="max-h-64"
              columns={CLAIM_COLUMNS}
              rows={claims}
              loading={claimsLoading}
              rowId={(c) => `${c.event_id}-${c.version}-${c.account_id}`}
              initialSort={{ column: 'claimed_at', direction: 'descending' }}
              sortValue={(c, k) => (c as unknown as Record<string, string | number>)[k] ?? ''}
              emptyState={(
                <EmptyState size="sm">
                  <EmptyState.Header>
                    <EmptyState.Title>{t('events.status.noClaims')}</EmptyState.Title>
                  </EmptyState.Header>
                </EmptyState>
              )}
              renderCell={(c, key) => {
                switch (key) {
                  case 'account_id':
                    return <span className="font-mono text-xs">{c.account_id}</span>
                  case 'version':
                    return <span className="font-mono text-xs text-muted">{c.version}</span>
                  case 'status':
                    return (
                      <Chip size="sm" variant="soft" color={claimStatusColor(c.status)}>
                        {claimStatusLabel(c.status)}
                      </Chip>
                    )
                  case 'attempts':
                    return <span className="text-muted text-xs">{c.attempts}</span>
                  case 'next_attempt_at':
                    return <span className="text-muted text-xs">{c.next_attempt_at || '—'}</span>
                  case 'claimed_at':
                    return <span className="text-muted text-xs">{c.claimed_at || '—'}</span>
                  case 'last_error':
                    return <span className="text-muted text-xs">{c.last_error || '—'}</span>
                  case 'actions':
                    return can('events:manage') && canGrantClaim(c.status)
                      ? (
                          <Button
                            size="sm"
                            variant="outline"
                            onPress={() => handleGrantClaim(c)}
                            aria-label={t('events.claims.grantAria', { account: c.account_id })}
                          >
                            {t('events.claims.grant')}
                          </Button>
                        )
                      : null
                }
              }}
            />
          </Panel>
        )}

        <EventEditorModal
          isOpen={editorOpen}
          onClose={() => setEditorOpen(false)}
          editing={editingEvent}
          onSaved={loadEvents}
        />
      </div>

      <ConfirmDialog
        open={deleteTarget !== null}
        title={t('events.confirmDeleteTitle')}
        description={t('events.confirmDelete', { name: deleteTarget?.name ?? '' })}
        confirmLabel={t('common.delete')}
        onConfirm={confirmDelete}
        onCancel={() => setDeleteTarget(null)}
      />
      <ConfirmDialog
        open={resetTarget !== null}
        title={t('events.confirmResetTitle')}
        description={t('events.confirmReset', { name: resetTarget?.name ?? '' })}
        confirmLabel={t('events.resetConfirmLabel')}
        onConfirm={confirmReset}
        onCancel={() => setResetTarget(null)}
      />

      <ActionBar aria-label={t('events.ariaLabel')} isOpen={can('events:manage') && selectionCount > 0}>
        <ActionBar.Prefix>
          <Chip size="sm" className="shrink-0 tabular-nums">{selectionCount}</Chip>
        </ActionBar.Prefix>
        <Separator />
        <ActionBar.Content>
          <Button
            size="sm"
            variant="ghost"
            className="text-danger"
            onPress={handleBulkDelete}
            aria-label={t('common.deleteSelected')}
          >
            <Icon name="trash-2" />
            <span className="action-bar__label">{t('common.deleteSelected')}</span>
          </Button>
        </ActionBar.Content>
        <Separator />
        <ActionBar.Suffix>
          <Button
            isIconOnly
            size="sm"
            variant="ghost"
            onPress={() => setSelectedKeys(new Set())}
            aria-label={t('common.clearSelection')}
          >
            <Icon name="x" />
          </Button>
        </ActionBar.Suffix>
      </ActionBar>
    </React.Fragment>
  )
}
