import { zodResolver } from '@hookform/resolvers/zod'
import { PermissionAction } from '@supabase/shared-types/out/constants'
import { useParams } from 'common'
import { Eye, EyeOff, Lock, Mail } from 'lucide-react'
import { useState } from 'react'
import { SubmitHandler, useForm } from 'react-hook-form'
import { toast } from 'sonner'
import {
  Button,
  Checkbox,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogSection,
  DialogSectionSeparator,
  DialogTitle,
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
  Input,
} from 'ui'
import * as z from 'zod'

import { useUserCreateMutation } from '@/data/auth/user-create-mutation'
import { useAsyncCheckPermissions } from '@/hooks/misc/useCheckPermissions'

export type CreateUserModalProps = {
  visible: boolean
  setVisible: (visible: boolean) => void
}

const CreateUserFormSchema = z.object({
  email: z.string().min(1, 'Email is required').email('Must be a valid email address'),
  password: z.string().min(1, 'Password is required'),
  autoConfirmUser: z.boolean(),
})

const CreateUserModal = ({ visible, setVisible }: CreateUserModalProps) => {
  const { ref: projectRef } = useParams()
  const [showPassword, setShowPassword] = useState(false)
  const { can: canCreateUsers } = useAsyncCheckPermissions(
    PermissionAction.AUTH_EXECUTE,
    'create_user'
  )

  const { mutate: createUser, isPending: isCreatingUser } = useUserCreateMutation({
    onSuccess(res) {
      toast.success(`Successfully created user: ${res.email}`)
      form.reset({ email: '', password: '', autoConfirmUser: true })
      setVisible(false)
    },
  })

  const onCreateUser: SubmitHandler<z.infer<typeof CreateUserFormSchema>> = async (values) => {
    if (!projectRef) return console.error('Project ref is required')
    createUser({ projectRef, user: values })
  }

  const form = useForm<z.infer<typeof CreateUserFormSchema>>({
    resolver: zodResolver(CreateUserFormSchema),
    defaultValues: { email: '', password: '', autoConfirmUser: true },
  })

  return (
    <Dialog open={visible} onOpenChange={setVisible}>
      <DialogContent size="small">
        <DialogHeader>
          <DialogTitle>Create a new user</DialogTitle>
        </DialogHeader>
        <DialogSectionSeparator />
        <Form {...form}>
          <form
            id="create-user"
            className="flex flex-col gap-y-5"
            onSubmit={form.handleSubmit(onCreateUser)}
            noValidate
          >
            <DialogSection className="flex flex-col gap-y-5" padding="small">
              <FormField
                name="email"
                control={form.control}
                render={({ field }) => (
                  <FormItem className="flex flex-col gap-1.5">
                    <FormLabel>Email address</FormLabel>
                    <FormControl>
                      <div className="relative">
                        <Mail
                          size={16}
                          className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-foreground-muted"
                          strokeWidth={1.5}
                        />
                        <Input
                          autoFocus
                          {...field}
                          autoComplete="email"
                          type="email"
                          name="email"
                          placeholder="user@example.com"
                          disabled={isCreatingUser}
                          className="h-10 pl-9"
                        />
                      </div>
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                name="password"
                control={form.control}
                render={({ field }) => (
                  <FormItem className="flex flex-col gap-1.5">
                    <FormLabel>User password</FormLabel>
                    <FormControl>
                      <div className="relative">
                        <Lock
                          size={16}
                          className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-foreground-muted"
                          strokeWidth={1.5}
                        />
                        <Input
                          {...field}
                          autoComplete="new-password"
                          type={showPassword ? 'text' : 'password'}
                          name="password"
                          placeholder="Enter a secure password"
                          disabled={isCreatingUser}
                          className="h-10 pl-9 pr-10"
                        />
                        <button
                          type="button"
                          aria-label={showPassword ? 'Hide password' : 'Show password'}
                          className="absolute right-2 top-1/2 flex h-7 w-7 -translate-y-1/2 items-center justify-center rounded text-foreground-muted transition-colors hover:bg-surface-200 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-foreground-muted"
                          onClick={() => setShowPassword((current) => !current)}
                          disabled={isCreatingUser}
                        >
                          {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                        </button>
                      </div>
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                name="autoConfirmUser"
                control={form.control}
                render={({ field }) => (
                  <FormItem className="flex items-start gap-x-2.5 pt-1">
                    <FormControl>
                      <Checkbox
                        checked={field.value}
                        onCheckedChange={(value) => field.onChange(value)}
                        className="mt-0.5"
                      />
                    </FormControl>
                    <div className="space-y-1">
                      <FormLabel className="cursor-pointer">Auto-confirm user</FormLabel>
                      <p className="text-xs leading-5 text-foreground-lighter">
                        Skip email verification for this account.
                      </p>
                    </div>
                  </FormItem>
                )}
              />
            </DialogSection>

            <DialogSectionSeparator />
            <DialogFooter padding="small" className="flex-col items-stretch gap-2">
              <p className="text-xs leading-5 text-foreground-lighter">
                No confirmation email will be sent when creating a user through this form.
              </p>
              <Button
                block
                size="small"
                type="submit"
                loading={isCreatingUser}
                disabled={!canCreateUsers || isCreatingUser}
                className="h-10"
              >
                Create user
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}

export default CreateUserModal
