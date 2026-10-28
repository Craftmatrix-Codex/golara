import { ArrowRight, Eye, EyeOff, KeyRound, Mail } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { Button, Input } from 'ui'

import { BASE_PATH } from '@/lib/constants'

const defaultDestination = `${BASE_PATH}/project/default`

function getReturnToPath(): string {
  const value = new URLSearchParams(window.location.search).get('returnTo')

  if (value && value.startsWith('/') && !value.startsWith('//') && !value.includes('\\')) {
    return value
  }

  return defaultDestination
}

export const GoLaraSignInPage = () => {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [isPasswordVisible, setIsPasswordVisible] = useState(false)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [errorMessage, setErrorMessage] = useState<string | null>(null)

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setIsSubmitting(true)
    setErrorMessage(null)

    try {
      const response = await fetch(`${BASE_PATH}/api/studio-auth/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
        body: JSON.stringify({ username, password }),
      })

      if (!response.ok) {
        if (response.status === 429) {
          setErrorMessage('Too many sign-in attempts. Wait a minute and try again.')
        } else if (response.status === 401) {
          setErrorMessage('Sign-in details not recognized. Check the email or username and password.')
        } else {
          setErrorMessage('Unable to sign in right now. Try again.')
        }
        return
      }

      window.location.replace(getReturnToPath())
    } catch {
      setErrorMessage('Unable to reach GoLara. Check your connection and try again.')
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <main className="relative isolate min-h-screen overflow-hidden bg-[#071827] text-white lg:grid lg:grid-cols-[1.08fr_0.92fr]">
      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-0 opacity-50"
        style={{
          backgroundImage:
            'linear-gradient(rgba(74, 135, 165, 0.10) 1px, transparent 1px), linear-gradient(90deg, rgba(74, 135, 165, 0.10) 1px, transparent 1px)',
          backgroundSize: '52px 52px',
          maskImage: 'linear-gradient(to bottom, black, transparent 92%)',
        }}
      />

      <section className="relative flex min-h-[250px] flex-col px-6 pt-5 sm:min-h-[290px] lg:min-h-screen lg:px-12 lg:pt-9 xl:px-16">
        <div className="relative z-10 inline-flex w-fit items-center gap-2.5">
          <img src={`${BASE_PATH}/img/golara-logo.svg`} alt="" className="h-9 w-9" />
          <span className="text-[25px] font-semibold tracking-tight">
            Go<span className="text-[#ff394b]">Lara</span>
          </span>
        </div>

        <div className="relative z-10 mt-10 hidden max-w-xl lg:block xl:mt-16">
          <h1 className="text-5xl font-semibold leading-[1.06] tracking-tight xl:text-6xl">
            Your backend,
            <br />
            <span className="text-[#ff394b]">ready for launch.</span>
          </h1>
          <p className="mt-5 max-w-lg text-lg leading-7 text-slate-300">
            Manage your projects, APIs, and data in one place.
          </p>
        </div>

        <div className="pointer-events-none absolute inset-x-0 top-[50px] h-[235px] overflow-hidden sm:top-[35px] sm:h-[280px] lg:static lg:mt-auto lg:flex lg:h-auto lg:flex-1 lg:items-end lg:justify-center">
          <img
            src={`${BASE_PATH}/img/golara-space-engineer.png`}
            alt="GoLara's space engineer mascot holding a diagnostic tablet"
            className="absolute left-1/2 top-0 h-[285px] max-w-none -translate-x-1/2 object-contain mix-blend-screen sm:h-[335px] lg:relative lg:left-auto lg:top-auto lg:h-[min(72vh,680px)] lg:max-w-full lg:translate-x-0"
          />
        </div>

        <p className="relative z-10 mb-7 mt-4 hidden text-sm text-slate-400 lg:block">
          Project-scoped backend tools for your next launch.
        </p>
      </section>

      <section className="relative z-10 -mt-1 min-h-[calc(100vh-250px)] rounded-t-[28px] bg-[#f7f9fc] px-5 pb-8 pt-8 text-slate-950 sm:min-h-[calc(100vh-290px)] sm:px-8 lg:mt-0 lg:flex lg:min-h-screen lg:items-center lg:justify-center lg:rounded-none lg:bg-[#0a1d2d] lg:px-10 lg:py-12">
        <div className="mx-auto w-full max-w-md rounded-3xl bg-white px-6 py-7 shadow-[0_24px_80px_rgba(0,0,0,0.22)] sm:px-9 sm:py-9 lg:px-10 lg:py-10">
          <header className="mb-7">
            <h2 className="text-[27px] font-semibold leading-tight tracking-tight sm:text-3xl">
              Sign in to GoLara
            </h2>
            <p className="mt-2 text-sm text-slate-500 sm:text-base">
              Use your deployment credentials to continue.
            </p>
          </header>

          <form className="space-y-5" onSubmit={handleSubmit}>
            <div>
              <label
                htmlFor="golara-username"
                className="mb-2 block text-sm font-medium text-slate-800"
              >
                Email or username
              </label>
              <div className="relative">
                <Mail
                  aria-hidden="true"
                  className="pointer-events-none absolute left-3.5 top-1/2 h-5 w-5 -translate-y-1/2 text-slate-400"
                />
                <Input
                  id="golara-username"
                  name="username"
                  autoComplete="username"
                  autoCapitalize="none"
                  required
                  value={username}
                  onChange={(event) => setUsername(event.target.value)}
                  placeholder="you@example.com"
                  className="h-12 rounded-lg border-slate-300 bg-white pl-11 text-base text-slate-900 placeholder:text-slate-400 focus-visible:ring-cyan-500"
                />
              </div>
            </div>

            <div>
              <label
                htmlFor="golara-password"
                className="mb-2 block text-sm font-medium text-slate-800"
              >
                Password
              </label>
              <div className="relative">
                <KeyRound
                  aria-hidden="true"
                  className="pointer-events-none absolute left-3.5 top-1/2 h-5 w-5 -translate-y-1/2 text-slate-400"
                />
                <Input
                  id="golara-password"
                  name="password"
                  type={isPasswordVisible ? 'text' : 'password'}
                  autoComplete="current-password"
                  required
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                  className="h-12 rounded-lg border-slate-300 bg-white pl-11 pr-12 text-base text-slate-900 focus-visible:ring-cyan-500"
                />
                <button
                  type="button"
                  onClick={() => setIsPasswordVisible((visible) => !visible)}
                  aria-label={isPasswordVisible ? 'Hide password' : 'Show password'}
                  className="absolute right-3 top-1/2 -translate-y-1/2 rounded p-1 text-slate-500 hover:text-slate-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-500"
                >
                  {isPasswordVisible ? (
                    <EyeOff aria-hidden="true" size={20} />
                  ) : (
                    <Eye aria-hidden="true" size={20} />
                  )}
                </button>
              </div>
            </div>

            {errorMessage && (
              <p
                role="alert"
                className="rounded-lg border border-red-200 bg-red-50 px-3.5 py-3 text-sm text-red-800"
              >
                {errorMessage}
              </p>
            )}

            <Button
              type="submit"
              block
              size="large"
              loading={isSubmitting}
              className="h-12 rounded-lg border-[#f1263d] bg-[#f1263d] text-base font-semibold text-white shadow-[0_8px_22px_rgba(241,38,61,0.22)] hover:border-[#d91c32] hover:bg-[#d91c32]"
            >
              Sign in
              <ArrowRight aria-hidden="true" className="ml-2 h-5 w-5" />
            </Button>
          </form>

          <p className="mt-6 border-t border-slate-200 pt-5 text-center text-sm leading-6 text-slate-500">
            GoLara access is managed by your deployment administrator.
          </p>
        </div>
      </section>
    </main>
  )
}
