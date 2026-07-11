<script setup>
import { computed, onMounted, ref } from 'vue'

const status = ref(null)
const components = ref([])
const incidents = ref({ active: [], recent: [] })
const loading = ref(true)
const error = ref('')

const statusTone = computed(() => toneFor(status.value?.overall?.status))
const activeIncidents = computed(() => incidents.value.active ?? [])
const recentResolved = computed(() =>
  (incidents.value.recent ?? []).filter((incident) => incident.status === 'resolved').slice(0, 4),
)

const groupedComponents = computed(() => {
  const groups = new Map()
  for (const component of components.value) {
    const group = component.group || 'Other'
    if (!groups.has(group)) groups.set(group, [])
    groups.get(group).push(component)
  }
  return Array.from(groups.entries()).map(([name, items]) => ({ name, items }))
})

const timelineComponents = computed(() =>
  components.value.map((component) => ({
    ...component,
    timeline: component.timeline ?? [],
  })),
)

const lowestUptime = computed(() => {
  const values = timelineComponents.value
    .map((component) => component.uptime90d)
    .filter((value) => typeof value === 'number')
  if (!values.length) return null
  return Math.min(...values)
})

onMounted(async () => {
  try {
    const [statusResponse, componentsResponse, incidentsResponse] = await Promise.all([
      fetch('/api/status.json'),
      fetch('/api/components.json'),
      fetch('/api/incidents.json'),
    ])

    for (const response of [statusResponse, componentsResponse, incidentsResponse]) {
      if (!response.ok) throw new Error(`${response.url} returned ${response.status}`)
    }

    const [statusPayload, componentsPayload, incidentsPayload] = await Promise.all([
      statusResponse.json(),
      componentsResponse.json(),
      incidentsResponse.json(),
    ])

    status.value = statusPayload
    components.value = componentsPayload.components ?? []
    incidents.value = incidentsPayload
  } catch (loadError) {
    error.value = loadError.message
  } finally {
    loading.value = false
  }
})

function toneFor(value) {
  return {
    operational: 'ok',
    degraded: 'warn',
    partial_outage: 'bad',
    major_outage: 'bad',
    maintenance: 'info',
    unknown: 'neutral',
  }[value] ?? 'info'
}

function formatDate(value) {
  if (!value) return 'Unknown'
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(new Date(value))
}

function formatDay(value) {
  if (!value) return ''
  return new Intl.DateTimeFormat(undefined, {
    month: 'short',
    day: 'numeric',
  }).format(new Date(`${value}T00:00:00Z`))
}

function uptimeLabel(value) {
  if (typeof value !== 'number') return 'No history'
  return `${value.toFixed(value >= 99.995 ? 0 : 2)}%`
}
</script>

<template>
  <main class="page-shell">
    <section class="status-board" v-if="!loading && !error">
      <header class="page-header">
        <div>
          <p class="eyebrow">Live service status</p>
          <h1>{{ status.page.name }}</h1>
          <p class="description">{{ status.page.description }}</p>
        </div>
        <div class="header-actions">
          <span class="current-pill" :class="`tone-${statusTone}`">
            <span></span>
            {{ status.overall.label }}
          </span>
          <div class="updated">
            <span>Updated</span>
            <strong>{{ formatDate(status.lastUpdated) }}</strong>
          </div>
        </div>
      </header>

      <section class="timeline-panel">
        <div class="section-heading timeline-heading">
          <div>
            <h2>Past 90 days</h2>
            <span>Daily status from static history and incidents</span>
          </div>
          <strong>{{ uptimeLabel(lowestUptime) }}</strong>
        </div>

        <div class="timeline-list">
          <article
            v-for="component in timelineComponents"
            :key="`${component.id}-timeline`"
            class="timeline-row"
          >
            <div class="timeline-label">
              <h3>{{ component.name }}</h3>
              <span>{{ uptimeLabel(component.uptime90d) }} uptime</span>
            </div>
            <div class="day-strip" :aria-label="`${component.name} 90 day history`">
              <span
                v-for="day in component.timeline"
                :key="`${component.id}-${day.date}`"
                class="day-cell"
                :class="`tone-${toneFor(day.status)}`"
                :title="`${formatDay(day.date)}: ${day.statusLabel}`"
              ></span>
            </div>
          </article>
        </div>

        <div class="timeline-footer">
          <span>{{ formatDay(timelineComponents[0]?.timeline?.[0]?.date) }}</span>
          <div class="legend">
            <span><i class="tone-ok"></i> Operational</span>
            <span><i class="tone-warn"></i> Degraded</span>
            <span><i class="tone-bad"></i> Outage</span>
            <span><i class="tone-neutral"></i> No data</span>
          </div>
          <span>Today</span>
        </div>
      </section>

      <section v-if="activeIncidents.length" class="active-incidents">
        <article v-for="incident in activeIncidents" :key="incident.id" class="incident-card active">
          <div class="incident-meta">
            <span>{{ incident.status }}</span>
            <time>{{ formatDate(incident.started_at) }}</time>
          </div>
          <h3>{{ incident.title }}</h3>
          <p>{{ incident.summary }}</p>
        </article>
      </section>

      <details class="details-panel">
        <summary>Component details and incident history</summary>
        <div class="detail-body">
          <section v-for="group in groupedComponents" :key="group.name" class="component-group">
            <h3>{{ group.name }}</h3>
            <article v-for="component in group.items" :key="component.id" class="component-row">
              <div>
                <h4>{{ component.name }}</h4>
                <p>{{ component.description }}</p>
              </div>
              <span class="mini-uptime">{{ uptimeLabel(component.uptime90d) }}</span>
              <span class="status-pill" :class="`tone-${toneFor(component.status)}`">
                <span></span>
                {{ component.statusLabel }}
              </span>
            </article>
          </section>

          <div v-if="recentResolved.length" class="incident-list">
            <article v-for="incident in recentResolved" :key="incident.id" class="incident-card">
              <div class="incident-meta">
                <span>{{ incident.status }}</span>
                <time>{{ formatDate(incident.resolved_at || incident.started_at) }}</time>
              </div>
              <h3>{{ incident.title }}</h3>
              <p>{{ incident.summary }}</p>
            </article>
          </div>
        </div>
      </details>
    </section>

    <section v-else-if="loading" class="loading-state">
      <div class="pulse"></div>
      <p>Loading status data...</p>
    </section>

    <section v-else class="loading-state error-state">
      <h1>Status data unavailable</h1>
      <p>{{ error }}</p>
    </section>
  </main>
</template>
