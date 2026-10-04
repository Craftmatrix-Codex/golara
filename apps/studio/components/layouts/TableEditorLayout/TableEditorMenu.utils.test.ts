import { describe, expect, it } from 'vitest'

import { getEntityNames, getValidEntities } from './TableEditorMenu.utils'

describe('getEntityNames', () => {
  it('ignores empty metadata rows returned by self-hosted pg-meta', () => {
    expect(getEntityNames([undefined, null, { name: 'users' }, { name: '' }])).toEqual(['users'])
  })
})

describe('getValidEntities', () => {
  it('removes nullish metadata rows before tabs and list rendering', () => {
    expect(getValidEntities([undefined, null, { id: 1, type: 'table' }])).toEqual([
      { id: 1, type: 'table' },
    ])
  })
})
