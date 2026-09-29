// The operator inbox that pilot requests go to. Unset until a real inbox
// exists; pages then show no email rather than an address that bounces.
export const contactEmail = (import.meta.env.VITE_PILOT_CONTACT_EMAIL || '').trim();

export type PilotRequest = {
  name: string;
  email: string;
  company: string;
  website: string;
  monthlyVisitors: string;
  botProblem: string;
  message: string;
};

// pilotRequestMailto turns the contact form into an email the visitor sends
// from their own mail app. There is no backend inbox for these requests, so
// this is the honest way to deliver them instead of pretending to submit.
export function pilotRequestMailto(to: string, r: PilotRequest): string {
  const body = [
    `Name: ${r.name}`,
    `Email: ${r.email}`,
    `Company: ${r.company}`,
    `Website: ${r.website}`,
    `Monthly visitors: ${r.monthlyVisitors}`,
    `Main bot problem: ${r.botProblem}`,
    '',
    r.message,
  ].join('\n');
  const subject = `HakaiShield pilot request: ${r.company || r.name}`;
  return `mailto:${encodeURIComponent(to)}?subject=${encodeURIComponent(subject)}&body=${encodeURIComponent(body)}`;
}
