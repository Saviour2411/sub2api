/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,js,ts,jsx,tsx}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        // 主色调 - 机甲冷蓝
        primary: {
          50: '#eff8ff',
          100: '#dbeeff',
          200: '#bee2ff',
          300: '#86ceff',
          400: '#4bb5ff',
          500: '#1798f2',
          600: '#0878d0',
          700: '#075ea9',
          800: '#0a4f8b',
          900: '#0e426f',
          950: '#092943'
        },
        // 辅助色 - 装甲石墨
        accent: {
          50: '#f7f9fc',
          100: '#e9eef5',
          200: '#d4deea',
          300: '#adbed3',
          400: '#7f98b3',
          500: '#5f7895',
          600: '#4a6078',
          700: '#3f5064',
          800: '#25303e',
          900: '#151c26',
          950: '#090f18'
        },
        // 深色模式背景
        dark: {
          50: '#f8fafc',
          100: '#f1f5f9',
          200: '#e2e8f0',
          300: '#cbd5e1',
          400: '#94a3b8',
          500: '#64748b',
          600: '#475569',
          700: '#334155',
          800: '#1e293b',
          900: '#0f172a',
          950: '#020617'
        }
      },
      fontFamily: {
        sans: [
          'system-ui',
          '-apple-system',
          'BlinkMacSystemFont',
          'Segoe UI',
          'Roboto',
          'Helvetica Neue',
          'Arial',
          'PingFang SC',
          'Hiragino Sans GB',
          'Microsoft YaHei',
          'sans-serif'
        ],
        mono: ['ui-monospace', 'SFMono-Regular', 'Menlo', 'Monaco', 'Consolas', 'monospace']
      },
      boxShadow: {
        glass: '0 16px 48px rgba(8, 47, 88, 0.12)',
        'glass-sm': '0 8px 22px rgba(8, 47, 88, 0.08)',
        glow: '0 0 24px rgba(23, 152, 242, 0.34)',
        'glow-lg': '0 0 54px rgba(23, 152, 242, 0.44)',
        card: '0 1px 3px rgba(0, 0, 0, 0.04), 0 1px 2px rgba(0, 0, 0, 0.06)',
        'card-hover': '0 18px 46px rgba(8, 47, 88, 0.16)',
        'inner-glow': 'inset 0 1px 0 rgba(255, 255, 255, 0.16)'
      },
      backgroundImage: {
        'gradient-radial': 'radial-gradient(var(--tw-gradient-stops))',
        'gradient-primary': 'linear-gradient(135deg, #35b8ff 0%, #0878d0 58%, #0b4c8a 100%)',
        'gradient-dark': 'linear-gradient(135deg, #172131 0%, #090f18 100%)',
        'gradient-glass':
          'linear-gradient(135deg, rgba(255,255,255,0.78) 0%, rgba(235,245,255,0.46) 100%)',
        'mesh-gradient':
          'linear-gradient(135deg, rgba(246,249,255,0.96) 0%, rgba(221,234,248,0.92) 46%, rgba(244,248,255,0.96) 100%), radial-gradient(at 80% 4%, rgba(23, 152, 242, 0.22) 0px, transparent 42%), radial-gradient(at 10% 86%, rgba(255, 111, 56, 0.12) 0px, transparent 38%), linear-gradient(115deg, transparent 0 46%, rgba(23,152,242,0.08) 46.2% 47%, transparent 47.2% 100%)'
      },
      animation: {
        'fade-in': 'fadeIn 0.3s ease-out',
        'slide-up': 'slideUp 0.3s ease-out',
        'slide-down': 'slideDown 0.3s ease-out',
        'slide-in-right': 'slideInRight 0.3s ease-out',
        'scale-in': 'scaleIn 0.2s ease-out',
        'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        shimmer: 'shimmer 2s linear infinite',
        glow: 'glow 2s ease-in-out infinite alternate'
      },
      keyframes: {
        fadeIn: {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' }
        },
        slideUp: {
          '0%': { opacity: '0', transform: 'translateY(10px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' }
        },
        slideDown: {
          '0%': { opacity: '0', transform: 'translateY(-10px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' }
        },
        slideInRight: {
          '0%': { opacity: '0', transform: 'translateX(20px)' },
          '100%': { opacity: '1', transform: 'translateX(0)' }
        },
        scaleIn: {
          '0%': { opacity: '0', transform: 'scale(0.95)' },
          '100%': { opacity: '1', transform: 'scale(1)' }
        },
        shimmer: {
          '0%': { backgroundPosition: '-200% 0' },
          '100%': { backgroundPosition: '200% 0' }
        },
        glow: {
          '0%': { boxShadow: '0 0 20px rgba(20, 184, 166, 0.25)' },
          '100%': { boxShadow: '0 0 30px rgba(20, 184, 166, 0.4)' }
        }
      },
      backdropBlur: {
        xs: '2px'
      },
      borderRadius: {
        '4xl': '2rem'
      }
    }
  },
  plugins: []
}
