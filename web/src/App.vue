<script setup>
import { computed, onMounted, ref } from 'vue'

const status = ref(null)
const components = ref([])
const incidents = ref({ active: [], recent: [] })
const loading = ref(true)
const error = ref('')
const activeDayKey = ref('')

const statusTone = computed(() => toneFor(status.value?.overall?.status))
const activeIncidents = computed(() => incidents.value.active ?? [])
const recentResolved = computed(() =>
  (incidents.value.recent ?? []).filter((incident) => incident.status === 'resolved').slice(0, 4),
)

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

const timelineRange = computed(() => {
  const first = timelineComponents.value[0]?.timeline?.[0]?.date
  const lastTimeline = timelineComponents.value[0]?.timeline
  const last = lastTimeline?.[lastTimeline.length - 1]?.date
  if (!first || !last) return ''
  return `${formatMonthYear(first)} - ${formatMonthYear(last)}`
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

function formatMonthYear(value) {
  return new Intl.DateTimeFormat(undefined, {
    month: 'short',
    year: 'numeric',
  }).format(new Date(`${value}T00:00:00Z`))
}

function uptimeLabel(value) {
  if (typeof value !== 'number') return 'No history'
  return `${value.toFixed(value >= 99.995 ? 0 : 2)}%`
}

function dayKey(component, day) {
  return `${component.id}-${day.date}`
}

function dayAriaLabel(component, day) {
  const incidentText = day.incidents?.length
    ? `${day.incidents.length} related incident${day.incidents.length === 1 ? '' : 's'}`
    : 'no incidents recorded'
  return `${component.name}, ${formatDay(day.date)}: ${day.statusLabel}, ${incidentText}`
}

function dayTitle(component, day) {
  return `${component.name} · ${formatDay(day.date)} · ${day.statusLabel} · ${dayIncidentSummary(day)}`
}

function dayIncidentSummary(day) {
  if (!day.incidents?.length) return 'No incidents recorded'
  if (day.incidents.length === 1) return day.incidents[0].title
  return `${day.incidents.length} related incidents`
}

function handleDayClick(day) {
  const incidentID = day.incidents?.[0]?.id
  if (!incidentID) return

  const target = document.getElementById(`incident-${incidentID}`)
  target?.scrollIntoView({ behavior: 'smooth', block: 'center' })
}

function statusGlyph(value) {
  if (value === 'operational') return ''
  if (value === 'maintenance') return 'i'
  return '!'
}
</script>

<template>
  <main class="page-shell">
    <section class="status-board" v-if="!loading && !error">
      <header class="page-header">
        <div class="brand">
          <img v-if="status.page.logo" :src="status.page.logo" alt="" />
          <h1>{{ status.page.name }}</h1>
        </div>
        <a v-if="status.page.contact" class="contact-button" :href="status.page.contact.url">
          {{ status.page.contact.label }}
        </a>
      </header>

      <section class="status-summary" :class="`tone-${statusTone}`">
        <span class="status-icon" aria-hidden="true">{{ statusGlyph(status.overall.status) }}</span>
        <h2>{{ status.overall.label }}</h2>
      </section>

      <section class="timeline-panel">
        <div class="section-heading timeline-heading">
          <div>
            <h2>System status</h2>
            <span>{{ timelineRange }}</span>
          </div>
          <strong>{{ uptimeLabel(lowestUptime) }} uptime</strong>
        </div>

        <div class="timeline-list">
          <article
            v-for="component in timelineComponents"
            :key="`${component.id}-timeline`"
            class="timeline-row"
          >
            <div class="timeline-row-header">
              <div class="timeline-label">
                <span class="row-status-dot" :class="`tone-${toneFor(component.status)}`">
                  {{ statusGlyph(component.status) }}
                </span>
                <h3>{{ component.name }}</h3>
                <span>{{ component.statusLabel }}</span>
              </div>
              <strong>{{ uptimeLabel(component.uptime90d) }} uptime</strong>
            </div>
            <div class="day-strip" :aria-label="`${component.name} 90 day history`">
              <button
                v-for="day in component.timeline"
                :key="`${component.id}-${day.date}`"
                type="button"
                class="day-cell"
                :class="[
                  `tone-${toneFor(day.status)}`,
                  {
                    clickable: day.incidents?.length,
                    'show-popover': activeDayKey === dayKey(component, day),
                  },
                ]"
                :aria-label="dayAriaLabel(component, day)"
                :title="dayTitle(component, day)"
                :tabindex="day.incidents?.length ? 0 : -1"
                @focus="activeDayKey = dayKey(component, day)"
                @blur="activeDayKey = ''"
                @mouseover="activeDayKey = dayKey(component, day)"
                @mouseout="activeDayKey = ''"
                @click="handleDayClick(day)"
              >
                <span class="day-popover" role="tooltip">
                  <strong>{{ formatDay(day.date) }}</strong>
                  <span>{{ day.statusLabel }}</span>
                  <em>{{ dayIncidentSummary(day) }}</em>
                </span>
              </button>
            </div>
          </article>
        </div>

      </section>

      <section v-if="activeIncidents.length" class="active-incidents">
        <h2>Active incidents</h2>
        <article
          v-for="incident in activeIncidents"
          :id="`incident-${incident.id}`"
          :key="incident.id"
          class="incident-card active"
        >
          <div class="incident-meta">
            <span>{{ incident.status }}</span>
            <time>{{ formatDate(incident.started_at) }}</time>
          </div>
          <h3>{{ incident.title }}</h3>
          <p>{{ incident.summary }}</p>
        </article>
      </section>

      <section v-if="recentResolved.length" class="past-incidents">
        <h2>Previous incidents</h2>
        <div class="incident-list">
          <article
            v-for="incident in recentResolved"
            :id="`incident-${incident.id}`"
            :key="incident.id"
            class="incident-card"
          >
            <div class="incident-meta">
              <span>{{ incident.status }}</span>
              <time>{{ formatDate(incident.resolved_at || incident.started_at) }}</time>
            </div>
            <h3>{{ incident.title }}</h3>
            <p>{{ incident.summary }}</p>
          </article>
        </div>
      </section>

      <footer class="page-footer">
        Powered by
        <a href="https://github.com/staatusHQ/staatus" target="_blank" rel="noreferrer">
          Staatus
        </a>
      </footer>
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
