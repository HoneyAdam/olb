import { type ClassValue, clsx } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

// Map a 0-100 value to closest Tailwind width class
export function wPct(v: number): string {
  if (v <= 5) return "w-[5%]"
  if (v <= 10) return "w-[10%]"
  if (v <= 15) return "w-[15%]"
  if (v <= 20) return "w-1/5"
  if (v <= 25) return "w-1/4"
  if (v <= 33) return "w-1/3"
  if (v <= 50) return "w-1/2"
  if (v <= 66) return "w-2/3"
  if (v <= 75) return "w-3/4"
  if (v <= 80) return "w-4/5"
  if (v <= 90) return "w-[90%]"
  return "w-full"
}
