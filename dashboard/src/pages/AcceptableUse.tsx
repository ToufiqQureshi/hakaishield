import LegalDocument from '../components/LegalDocument';
import { acceptableUseSections } from '../lib/legal';

export default function AcceptableUse() { return <LegalDocument title="Acceptable Use Policy" updated="2026-10-05" sections={acceptableUseSections} />; }
