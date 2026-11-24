/** Join class names, skipping falsy values. (Plain module so server and client components can both use it.) */
export function cx(...a: (string | false | null | undefined)[]): string {
  return a.filter(Boolean).join(" ");
}
