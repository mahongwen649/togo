import { mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useDeviceMode } from '../useDeviceMode'

const DeviceProbe = defineComponent({
  setup() {
    return useDeviceMode()
  },
  template: `
    <div
      :data-mobile-device="String(isMobileDevice)"
      :data-mobile-h5="String(isMobileH5)"
      :data-compact-navigation="String(isCompactNavigation)"
      :data-desktop="String(isDesktop)"
    />
  `,
})

function stubNavigator(userAgent: string, maxTouchPoints = 0) {
  Object.defineProperty(window.navigator, 'userAgent', {
    configurable: true,
    value: userAgent,
  })
  Object.defineProperty(window.navigator, 'maxTouchPoints', {
    configurable: true,
    value: maxTouchPoints,
  })
}

function stubViewport(width: number) {
  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    writable: true,
    value: vi.fn((query: string) => {
      const maxWidth = query.match(/max-width:\s*(\d+)px/)
      const minWidth = query.match(/min-width:\s*(\d+)px/)
      const matches = maxWidth
        ? width <= Number(maxWidth[1])
        : minWidth
          ? width >= Number(minWidth[1])
          : query.includes('pointer: fine') || query.includes('hover: hover')

      return {
        matches,
        media: query,
        onchange: null,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        addListener: vi.fn(),
        removeListener: vi.fn(),
        dispatchEvent: vi.fn(),
      }
    }),
  })
}

describe('useDeviceMode', () => {
  beforeEach(() => {
    stubNavigator('Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/126.0 Safari/537.36')
    stubViewport(1440)
  })

  it('enables H5 mode for a mobile device', () => {
    stubNavigator('Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) Mobile/15E148', 5)
    stubViewport(390)

    const attributes = mount(DeviceProbe).attributes()
    expect(attributes['data-mobile-device']).toBe('true')
    expect(attributes['data-mobile-h5']).toBe('true')
    expect(attributes['data-compact-navigation']).toBe('true')
    expect(attributes['data-desktop']).toBe('false')
  })

  it('keeps PC mode when a desktop browser is narrow', () => {
    stubViewport(390)

    const attributes = mount(DeviceProbe).attributes()
    expect(attributes['data-mobile-device']).toBe('false')
    expect(attributes['data-mobile-h5']).toBe('false')
    expect(attributes['data-compact-navigation']).toBe('false')
    expect(attributes['data-desktop']).toBe('true')
  })

  it('keeps H5 mode for a mobile device in landscape or a wide viewport', () => {
    stubNavigator('Mozilla/5.0 (Android 15; Mobile) AppleWebKit/537.36', 5)
    stubViewport(1280)

    const attributes = mount(DeviceProbe).attributes()
    expect(attributes['data-mobile-device']).toBe('true')
    expect(attributes['data-mobile-h5']).toBe('true')
    expect(attributes['data-compact-navigation']).toBe('true')
    expect(attributes['data-desktop']).toBe('false')
  })
})
