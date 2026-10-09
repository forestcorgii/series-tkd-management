/** @type {import('tailwindcss').Config} */
module.exports = {
  darkMode: 'class',
  content: [
    "./web/templates/**/*.html",
    "./web/static/**/*.js",
    "./internal/**/*.go"
  ],
  theme: {
    extend: {
      fontFamily: {
        sans: ['Montserrat', 'sans-serif'],
        display: ['Outfit', '"Good Timing"', 'sans-serif'],
      },
      colors: {
        brand: {
          black: '#000000',
          red: '#990303',
          'red-hover': '#7D0202',
          'red-dark': '#DC2626',
          'red-dark-hover': '#B91C1C',
          cream: '#FFFDF4',
          'cream-dark': '#F4F1E4',
          border: '#E5E3D8',
          muted: '#666666',
          success: '#1B7F43',
          warning: '#C98A0C',
        }
      }
    }
  },
  plugins: [],
}
