import { describe, expect, it } from 'vitest'

import { getEntityNames } from './TableEditorMenu.utils'

describe('getEntityNames', () => {
  it('ignores empty metadata rows returned by self-hosted pg-meta', () => {
    expect(getEntityNames([undefined, null, { name: 'users' }, { name: '' }])).toEqual(['users'])
  })
})
