<script lang="ts">
  import { cn } from '$lib/utils';

  type Props = {
    class?: string;
    checked?: boolean;
    disabled?: boolean;
    id?: string;
    name?: string;
    onCheckedChange?: (checked: boolean) => void;
  };

  let {
    class: className,
    checked: checkedProp,
    disabled = false,
    id,
    name,
    onCheckedChange,
  }: Props = $props();

  let uncontrolled = $state(false);
  let checked = $derived.by((): boolean =>
    checkedProp !== undefined ? checkedProp : uncontrolled
  );
  let switchState = $derived<'checked' | 'unchecked'>(checked ? 'checked' : 'unchecked');

  function handleChange(e: Event) {
    const next = (e.target as HTMLInputElement).checked;
    if (checkedProp === undefined) {
      uncontrolled = next;
    }
    onCheckedChange?.(next);
  }
</script>

<label
  data-state={switchState}
  class={cn(
    'inline-flex h-[24px] w-[44px] shrink-0 cursor-pointer items-center rounded-full border-2 border-transparent transition-colors',
    'has-[:disabled]:cursor-not-allowed has-[:disabled]:opacity-50',
    'focus-within:outline-none focus-within:ring-2 focus-within:ring-ring focus-within:ring-offset-2 focus-within:ring-offset-background',
    'data-[state=checked]:bg-primary data-[state=unchecked]:bg-input',
    className
  )}
>
  <input
    type="checkbox"
    role="switch"
    {id}
    {name}
    {checked}
    {disabled}
    class="peer sr-only"
    onchange={handleChange}
  />
  <span
    data-state={switchState}
    class="pointer-events-none block h-5 w-5 rounded-full bg-background shadow-lg ring-0 transition-transform data-[state=checked]:translate-x-5 data-[state=unchecked]:translate-x-0"
  ></span>
</label>
