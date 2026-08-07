type IconName = 'activity' | 'alert' | 'box' | 'check' | 'cloud' | 'code' | 'dollar' | 'info' | 'message' | 'server' | 'trend-down' | 'trend-up' | 'wrench'

const paths: Record<IconName, string> = {
  activity: 'M3 12h4l2-7 4 14 2-7h6', alert: 'M12 9v4m0 4h.01M10.3 3.7 2.4 18a2 2 0 0 0 1.8 3h15.6a2 2 0 0 0 1.8-3L13.7 3.7a2 2 0 0 0-3.4 0Z', box: 'm4 7 8-4 8 4-8 4-8-4Zm0 0v10l8 4 8-4V7m-8 4v10', check: 'm5 12 4 4L19 6', cloud: 'M17.5 19H6a4 4 0 1 1 1.2-7.8A6 6 0 0 1 18.7 13 3 3 0 0 1 17.5 19Z', code: 'm8 9-3 3 3 3m8-6 3 3-3 3m-3-8-2 10', dollar: 'M12 2v20m5-16.5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H7', info: 'M12 16v-4m0-4h.01M22 12a10 10 0 1 1-20 0 10 10 0 0 1 20 0Z', message: 'M21 15a4 4 0 0 1-4 4H8l-5 3V7a4 4 0 0 1 4-4h10a4 4 0 0 1 4 4Z', server: 'M4 4h16v6H4V4Zm0 10h16v6H4v-6Zm3-7h.01M7 17h.01', 'trend-down': 'm3 7 6 6 4-4 8 8m0 0v-6m0 6h-6', 'trend-up': 'm3 17 6-6 4 4 8-8m0 0h-6m6 0v6', wrench: 'M14.7 6.3a4 4 0 0 0-5-5L12 3.6 9.6 6 7.3 3.7a4 4 0 0 0 5 5L4 17a2.1 2.1 0 1 0 3 3l8.3-8.3a4 4 0 0 0 5-5L18 9l-3-3 2.3-2.3a4 4 0 0 0-2.6 2.6Z',
}

export function Icon({ name, size = 18, className = '' }: { name: IconName; size?: number; className?: string }) {
  return <svg aria-hidden="true" className={className} width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><path d={paths[name]} /></svg>
}
