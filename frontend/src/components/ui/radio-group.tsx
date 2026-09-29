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
  const ctx = React.useMemo(
    () => ({ value, onValueChange, name }),
    [value, onValueChange, name],
  )
  return (
    <RadioGroupContext.Provider value={ctx}>
      <div
        role="radiogroup"
        data-slot="radio-group"
        className={cn("grid gap-2", className)}
        {...props}
      >
        {children}
      </div>
    </RadioGroupContext.Provider>
  )
}

/** 原生 radio：用 border 宽度模拟选中圆点，避免额外伪元素与嵌套节点 */
function RadioGroupItem({
  className,
  value,
  ...props
}: Omit<React.ComponentProps<"input">, "value" | "type"> & { value: string }) {
  const { value: selected, onValueChange, name } = React.useContext(RadioGroupContext)
  return (
    <input
      type="radio"
      name={name}
      value={value}
      checked={selected === value}
      onChange={() => onValueChange?.(value)}
      data-slot="radio-group-item"
      className={cn(
        "size-4 shrink-0 cursor-pointer appearance-none rounded-full border border-input bg-background transition-[border-color,border-width] outline-none",
        "checked:border-4 checked:border-primary",
        "focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50",
        "disabled:cursor-not-allowed disabled:opacity-50",
        className,
      )}
      {...props}
    />
  )
}

export { RadioGroup, RadioGroupItem }