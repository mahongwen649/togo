import { DriveStep } from 'driver.js'

/**
 * 管理员引导流程
 * 交互式引导：只指向当前 Portal 保留的真实页面/控件。
 * Core 负责实际请求执行与调度，Portal 侧不再引导管理员维护执行资源。
 * @param t 国际化函数
 * @param _isSimpleMode 保留旧调用签名；当前管理员引导不再包含需按模式过滤的旧执行资源步骤
 */
export const getAdminSteps = (t: (key: string) => string, _isSimpleMode = false): DriveStep[] => [
  // ========== 欢迎介绍 ==========
  {
    popover: {
      title: t('onboarding.admin.welcome.title'),
      description: t('onboarding.admin.welcome.description'),
      align: 'center',
      nextBtnText: t('onboarding.admin.welcome.nextBtn'),
      prevBtnText: t('onboarding.admin.welcome.prevBtn')
    }
  },

  // ========== 创建 API 密钥 ==========
  {
    element: '[data-tour="sidebar-my-keys"]',
    popover: {
      title: t('onboarding.admin.keyManage.title'),
      description: t('onboarding.admin.keyManage.description'),
      side: 'right',
      align: 'center',
      showButtons: ['close']
    }
  },
  {
    element: '[data-tour="keys-create-btn"]',
    popover: {
      title: t('onboarding.admin.createKey.title'),
      description: t('onboarding.admin.createKey.description'),
      side: 'bottom',
      align: 'end',
      showButtons: ['close']
    }
  },
  {
    element: '[data-tour="key-form-name"]',
    popover: {
      title: t('onboarding.admin.keyName.title'),
      description: t('onboarding.admin.keyName.description'),
      side: 'right',
      align: 'start',
      showButtons: ['next', 'previous']
    }
  },
  {
    element: '[data-tour="key-form-group"]',
    popover: {
      title: t('onboarding.admin.keyGroup.title'),
      description: t('onboarding.admin.keyGroup.description'),
      side: 'right',
      align: 'start',
      showButtons: ['next', 'previous']
    }
  },
  {
    element: '[data-tour="key-form-submit"]',
    popover: {
      title: t('onboarding.admin.keySubmit.title'),
      description: t('onboarding.admin.keySubmit.description'),
      side: 'left',
      align: 'center',
      showButtons: ['close']
    }
  }
]

/**
 * 普通用户引导流程
 */
export const getUserSteps = (t: (key: string) => string): DriveStep[] => [
  {
    popover: {
      title: t('onboarding.user.welcome.title'),
      description: t('onboarding.user.welcome.description'),
      align: 'center',
      nextBtnText: t('onboarding.user.welcome.nextBtn'),
      prevBtnText: t('onboarding.user.welcome.prevBtn')
    }
  },
  {
    element: '[data-tour="sidebar-my-keys"]',
    popover: {
      title: t('onboarding.user.keyManage.title'),
      description: t('onboarding.user.keyManage.description'),
      side: 'right',
      align: 'center',
      showButtons: ['close']
    }
  },
  {
    element: '[data-tour="keys-create-btn"]',
    popover: {
      title: t('onboarding.user.createKey.title'),
      description: t('onboarding.user.createKey.description'),
      side: 'bottom',
      align: 'end',
      showButtons: ['close']
    }
  },
  {
    element: '[data-tour="key-form-name"]',
    popover: {
      title: t('onboarding.user.keyName.title'),
      description: t('onboarding.user.keyName.description'),
      side: 'right',
      align: 'start',
      showButtons: ['next', 'previous']
    }
  },
  {
    element: '[data-tour="key-form-group"]',
    popover: {
      title: t('onboarding.user.keyGroup.title'),
      description: t('onboarding.user.keyGroup.description'),
      side: 'right',
      align: 'start',
      showButtons: ['next', 'previous']
    }
  },
  {
    element: '[data-tour="key-form-submit"]',
    popover: {
      title: t('onboarding.user.keySubmit.title'),
      description: t('onboarding.user.keySubmit.description'),
      side: 'left',
      align: 'center',
      showButtons: ['close']
    }
  }
]
