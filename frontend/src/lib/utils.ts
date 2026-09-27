import { cn } from 'cn';

export { cn };

/*
 * The element-ref type helpers shadcn-svelte's generated primitives import from
 * this file (they are not exported by the `cn` package). Kept identical to what
 * `shadcn-svelte add` writes, so regenerating a primitive keeps compiling.
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export type WithoutChild<T> = T extends { child?: any } ? Omit<T, 'child'> : T;
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export type WithoutChildren<T> = T extends { children?: any } ? Omit<T, 'children'> : T;
export type WithoutChildrenOrChild<T> = WithoutChildren<WithoutChild<T>>;
export type WithElementRef<T, U extends HTMLElement = HTMLElement> = T & { ref?: U | null };
