import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const viewPath = resolve(dirname(fileURLToPath(import.meta.url)), '../KeysView.vue')
const viewSource = readFileSync(viewPath, 'utf8')

describe('KeysView create modal', () => {
  it('shows the API endpoint row on the keys page with a fallback base URL', () => {
    expect(viewSource).toMatch(/<template #filters>[\s\S]*<EndpointPopover[\s\S]*:api-base-url="primaryApiBaseUrl"/)
    expect(viewSource).not.toContain('v-if="publicSettings?.api_base_url || (publicSettings?.custom_endpoints?.length ?? 0) > 0"')
  })

  it('opens create modal through a helper that defaults to the first available group', () => {
    expect(viewSource).toContain('@click="openCreateModal"')
    expect(viewSource).toContain('function selectDefaultGroupForCreate()')
    expect(viewSource).toContain('formData.value.group_id = groupOptions.value[0]?.value ?? null')
  })


  it('shows API endpoints in the create-key modal for copying', () => {
    expect(viewSource).toMatch(/<EndpointPopover[\s\S]*v-if="!showEditModal"/)
    expect(viewSource).toContain(':api-base-url="primaryApiBaseUrl"')
		expect(viewSource).toContain("const primaryApiBaseUrl = computed(() => publicSettings.value?.api_base_url || getPublicGatewayBaseURL())")
  })
})
