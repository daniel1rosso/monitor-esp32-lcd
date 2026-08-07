import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'

void i18n.use(initReactI18next).init({
  lng: 'es-AR',
  fallbackLng: 'es-AR',
  interpolation: { escapeValue: false },
  resources: {
    'es-AR': {
      translation: {
        status: 'Arquitectura lista',
        title: 'Centro de monitoreo',
        description:
          'La base visual está preparada. Los módulos operativos llegarán después del gate de infraestructura.',
      },
    },
  },
})

export default i18n

