import { useState, useEffect, useCallback } from 'react'
import { api } from '@/lib/api'
import { toast } from 'sonner'

interface UseQueryOptions<T> {
  onSuccess?: (data: T) => void
  onError?: (error: Error) => void
  enabled?: boolean
  refetchInterval?: number
}

interface QueryResult<T> {
  data: T | null
  isLoading: boolean
  error: Error | null
  refetch: () => Promise<void>
}

export function useQuery<T>(
  queryFn: () => Promise<{ success: boolean; data: T }>,
  options: UseQueryOptions<T> = {}
): QueryResult<T> {
  const { onSuccess, onError, enabled = true, refetchInterval } = options
  const [data, setData] = useState<T | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<Error | null>(null)

  const fetch = useCallback(async () => {
    if (!enabled) return

    try {
      setIsLoading(true)
      setError(null)
      const response = await queryFn()
      if (response.success) {
        setData(response.data)
        onSuccess?.(response.data)
      }
    } catch (err) {
      const error = err instanceof Error ? err : new Error('Unknown error')
      setError(error)
      onError?.(error)
    } finally {
      setIsLoading(false)
    }
  }, [queryFn, enabled, onSuccess, onError])

  useEffect(() => {
    fetch()
  }, [fetch])

  useEffect(() => {
    if (!refetchInterval || !enabled) return
    const interval = setInterval(fetch, refetchInterval)
    return () => clearInterval(interval)
  }, [fetch, refetchInterval, enabled])

  return { data, isLoading, error, refetch: fetch }
}

interface UseMutationOptions<T, V> {
  onSuccess?: (data: T, variables: V) => void
  onError?: (error: Error, variables: V) => void
}

interface MutationResult<T, V> {
  mutate: (variables: V) => Promise<T | undefined>
  isLoading: boolean
  error: Error | null
  data: T | null
}

export function useMutation<T, V = void>(
  mutationFn: (variables: V) => Promise<T>,
  options: UseMutationOptions<T, V> = {}
): MutationResult<T, V> {
  const { onSuccess, onError } = options
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<Error | null>(null)
  const [data, setData] = useState<T | null>(null)

  const mutate = useCallback(async (variables: V): Promise<T | undefined> => {
    try {
      setIsLoading(true)
      setError(null)
      const result = await mutationFn(variables)
      setData(result)
      onSuccess?.(result, variables)
      return result
    } catch (err) {
      const error = err instanceof Error ? err : new Error('Unknown error')
      setError(error)
      onError?.(error, variables)
      throw error
    } finally {
      setIsLoading(false)
    }
  }, [mutationFn, onSuccess, onError])

  return { mutate, isLoading, error, data }
}

// Health query hook
export function useHealth(options?: UseQueryOptions<{ status: string; checks: Record<string, {status: string; message: string}>; timestamp: string }>) {
  return useQuery(() => api.getHealth(), {
    refetchInterval: 30000,
    ...options
  })
}

// System info query hook
export function useSystemInfo(options?: UseQueryOptions<{ version: string; commit: string; build_date: string; uptime: string; state: string; go_version: string }>) {
  return useQuery(() => api.getInfo(), {
    refetchInterval: 60000,
    ...options
  })
}

// Pools query hook
export function usePools(options?: UseQueryOptions<any[]>) {
  return useQuery(() => api.getPools().then(r => ({ success: true, data: r })), options)
}

// Listeners query hook
export function useListeners(options?: UseQueryOptions<any[]>) {
  return useQuery(() => api.getListeners().then(r => ({ success: true, data: r })), options)
}

// Toast notifications for mutations
export function useToastMutation<T, V = void>(
  mutationFn: (variables: V) => Promise<T>,
  options: {
    loadingMessage?: string
    successMessage?: string | ((data: T) => string)
    errorMessage?: string | ((error: Error) => string)
  } & UseMutationOptions<T, V> = {}
): MutationResult<T, V> {
  const { loadingMessage, successMessage, errorMessage, ...rest } = options

  return useMutation(mutationFn, {
    ...rest,
    onSuccess: (data, variables) => {
      if (successMessage) {
        const message = typeof successMessage === 'function' ? successMessage(data) : successMessage
        toast.success(message)
      }
      options.onSuccess?.(data, variables)
    },
    onError: (error, variables) => {
      if (errorMessage) {
        const message = typeof errorMessage === 'function' ? errorMessage(error) : errorMessage
        toast.error(message)
      } else {
        toast.error(error.message || 'An error occurred')
      }
      options.onError?.(error, variables)
    },
  })
}
