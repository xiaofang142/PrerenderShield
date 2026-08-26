import { describe, it } from 'vitest'
import i18nRaw from '../i18n'
import { render } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import Dashboard from '../pages/Dashboard'

describe('probe4', () => {
  it('repro dashboard render', async () => {
    await i18nRaw.changeLanguage('zh')
    console.log('lang:', i18nRaw.language, 't:', i18nRaw.t('dashboard.title'))
    const { container } = render(<MemoryRouter><Dashboard /></MemoryRouter>)
    console.log('h1:', container.querySelector('.page-title')?.textContent)
  })
})
