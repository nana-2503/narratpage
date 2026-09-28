import * as React from "react"
import { cn } from "cn"

interface RadioGroupContextValue {
  value?: string
  onValueChange?: (value: string) => void
  name?: string
}

const RadioGroupContext = React.createContext<RadioGroupContextValue>({})

function RadioGroup({
  className,
  value,
  onValueChange,
  name,
  children,
  ...props
}: React.ComponentProps<"div"> & RadioGroupContextValue) {
  return (
    <RadioGroupContext.Provider value={{ value, onValueChange, name }}>
      <div
        role="radiogroup"
        data-slot="radio-group"
        className={cn("grid gap-3", className)}
        {...props}
      >
        {children}
      </div>
    </RadioGroupContext.Provider>
  )
}

function RadioGroupItem({
  className,
  value,
  ...props
}: React.ComponentProps<"input"> & { value: string }) {
  const ctx = React.useContext(RadioGroupContext)
  const checked = ctx.value === value
  return (
    <div
      data-slot="radio-group-item"
      className={cn(
        "border-primary text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50",
        className,
      )}
    >
      <input
        type="radio"
        className="size-4 rounded-full border border-primary"
        checked={checked}
        {...props}
        onChange={(e) => {
          if (e.target.checked) {
            ctx.onValueChange?.(value)
          }
        }}
      />
    </div>
  )
}

export { RadioGroup, RadioGroupItem }
