// Where a signed-out visitor goes. The site root is the public front door,
// so it shows the landing page; any other dashboard URL needs a sign-in.
export function signedOutPath(pathname: string): string {
  return pathname === '/' || pathname === '' ? '/landing' : '/sign-in';
}
