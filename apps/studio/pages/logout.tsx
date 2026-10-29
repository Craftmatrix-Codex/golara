import { useRouter } from 'next/router'
import { useCallback, useEffect, useState } from 'react'
import { toast } from 'sonner'
import { Button, LogoLoader } from 'ui'

import { useSignOut } from '@/lib/auth'
import { BASE_PATH, IS_PLATFORM } from '@/lib/constants'
import type { NextPageWithLayout } from '@/types'

const LogoutPage: NextPageWithLayout = () => {
  const router = useRouter()
  const signOut = useSignOut()
  const [errorMessage, setErrorMessage] = useState<string | null>(null)

  const logout = useCallback(async () => {
    setErrorMessage(null)
    try {
      if (IS_PLATFORM) {
        const result = await signOut()
        if (result.error) throw result.error
      } else {
        const response = await fetch(`${BASE_PATH}/api/studio-auth/logout`, {
          method: 'POST',
          headers: { Accept: 'application/json' },
        })
        if (!response.ok) throw new Error('Studio logout failed')
      }
      toast('Successfully logged out')
      await router.replace('/sign-in')
    } catch {
      setErrorMessage('Unable to sign out. Try again.')
    }
  }, [router, signOut])

  useEffect(() => {
    void logout()
  }, [logout])

  return (
    <div className="w-full h-screen flex items-center justify-center">
      {errorMessage ? (
        <div className="flex max-w-sm flex-col items-center gap-3 p-6 text-center">
          <p role="alert" className="text-sm text-foreground-light">
            {errorMessage}
          </p>
          <Button onClick={() => void logout()}>Try again</Button>
        </div>
      ) : (
        <LogoLoader />
      )}
    </div>
  )
}

export default LogoutPage
