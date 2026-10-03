import { describe, expect, it } from 'vitest'

import { getUserNameParts } from './Users.utils'

const user = (metadata: Record<string, unknown>) =>
  ({ raw_user_meta_data: metadata, providers: [], id: 'user-1' }) as any

describe('getUserNameParts', () => {
  it('reads first, middle, and last names from user metadata', () => {
    expect(
      getUserNameParts(user({ first_name: 'Ada', middle_name: 'Lovelace', last_name: 'Byron' }))
    ).toEqual({ firstName: 'Ada', middleName: 'Lovelace', lastName: 'Byron' })
  })

  it('falls back to splitting a full name when explicit parts are absent', () => {
    expect(getUserNameParts(user({ full_name: 'Ada Lovelace Byron' }))).toEqual({
      firstName: 'Ada',
      middleName: 'Lovelace',
      lastName: 'Byron',
    })
  })
})
