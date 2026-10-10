// A real support inbox must be configured before the contact form is enabled.
export const contactEmail = (import.meta.env.VITE_CONTACT_EMAIL || import.meta.env.VITE_PILOT_CONTACT_EMAIL || '').trim();

export type ContactRequest = {
  name: string;
  email: string;
  company: string;
  website: string;
  monthlyVisitors: string;
  botProblem: string;
  message: string;
};

// The visitor sends this from their mail app. There is no backend inbox, so
// opening a mail draft must never be represented as a completed submission.
export function contactRequestMailto(to: string, r: ContactRequest): string {
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
  const subject = `HakaiShield inquiry: ${r.company || r.name}`;
  return `mailto:${encodeURIComponent(to)}?subject=${encodeURIComponent(subject)}&body=${encodeURIComponent(body)}`;
}
