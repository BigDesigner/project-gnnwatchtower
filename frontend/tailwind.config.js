/** @type {import('tailwindcss').Config} */
export default {
  darkMode: 'class',
  content: [
    "./index.html",
    "./src/**/*.{vue,js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        canvas: '#0c0e12',
        card: '#141820',
        subtle: '#181e28',
        hover: '#1d2432',
        border: '#232936',
        borderLight: '#2c3547',
        brand: {
          50: '#f0f7ff',
          500: '#3b82f6',
          600: '#2563eb',
        },
        txt: {
          primary: '#e6edf3',
          secondary: '#94a3b8',
          muted: '#64748b',
        }
      },
      fontFamily: {
        sans: ['Inter', 'system-ui', 'sans-serif'],
        mono: ['JetBrains Mono', 'monospace'],
      }
    },
  },
  plugins: [],
}
