// 主题配置
export const themes = {
  light: {
    token: {
      colorPrimary: '#2f855a',
      colorPrimaryHover: '#389e6d',
      colorPrimaryActive: '#1d684f',
      colorBgLayout: '#ffffff',
      colorBgContainer: '#ffffff',
      colorBgElevated: '#ffffff',
      colorTextPrimary: '#000000',
      colorTextSecondary: '#666666',
      colorTextTertiary: '#999999',
      colorBorder: '#e8e8e8',
      colorSplit: '#e8e8e8',
      borderRadius: 6,
      boxShadow: '0 2px 8px rgba(0, 0, 0, 0.1)',
    },
    components: {
      Layout: {
        headerBg: '#ffffff',
        siderBg: '#ffffff',
        bodyBg: '#f5f5f5',
        triggerBg: '#ffffff',
      },
      Menu: {
        itemBg: '#ffffff',
        subMenuItemBg: '#ffffff',
      },
      Card: {
        colorBgContainer: '#ffffff',
      },
    },
  },
  dark: {
    token: {
      colorPrimary: '#52c41a',
      colorPrimaryHover: '#73d13d',
      colorColorPrimaryActive: '#389e0d',
      colorBgLayout: '#141414',
      colorBgContainer: '#1f1f1f',
      colorBgElevated: '#262626',
      colorTextPrimary: '#ffffff',
      colorTextSecondary: '#bfbfbf',
      colorTextTertiary: '#8c8c8c',
      colorBorder: '#424242',
      colorSplit: '#424242',
      borderRadius: 6,
      boxShadow: '0 2px 8px rgba(0, 0, 0, 0.3)',
    },
    components: {
      Layout: {
        headerBg: '#1f1f1f',
        siderBg: '#1f1f1f',
        bodyBg: '#141414',
        triggerBg: '#1f1f1f',
      },
      Menu: {
        itemBg: '#1f1f1f',
        subMenuItemBg: '#1f1f1f',
        darkItemBg: '#1f1f1f',
      },
      Card: {
        colorBgContainer: '#1f1f1f',
      },
    },
  },
};

// 获取主题
export const getTheme = (isDark: boolean) => {
  return isDark ? themes.dark : themes.light;
};

// 检测系统主题
export const detectSystemTheme = (): boolean => {
  if (typeof window !== 'undefined' && window.matchMedia) {
    return window.matchMedia('(prefers-color-scheme: dark)').matches;
  }
  return false;
};

// 本地存储主题
export const getStoredTheme = (): boolean => {
  if (typeof window !== 'undefined') {
    const stored = localStorage.getItem('theme');
    if (stored) {
      return stored === 'dark';
    }
    return detectSystemTheme();
  }
  return false;
};

// 保存主题
export const setStoredTheme = (isDark: boolean) => {
  if (typeof window !== 'undefined') {
    localStorage.setItem('theme', isDark ? 'dark' : 'light');
    document.documentElement.setAttribute('data-theme', isDark ? 'dark' : 'light');
  }
};
