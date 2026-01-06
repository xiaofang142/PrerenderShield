import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import LanguageDetector from 'i18next-browser-languagedetector';

import zh from './locales/zh.json';
import en from './locales/en.json';

// TODO: Add translations for ar, fr, ru, es
const resources = {
  zh: { translation: zh },
  en: { translation: en },
  ar: { translation: { common: { loading: "جاري التحميل..." } } },
  fr: { translation: { common: { loading: "Chargement..." } } },
  ru: { translation: { common: { loading: "Загрузка..." } } },
  es: { translation: { common: { loading: "Cargando..." } } }
};

i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources,
    fallbackLng: 'zh',
    interpolation: {
      escapeValue: false,
    },
  });

export default i18n;
