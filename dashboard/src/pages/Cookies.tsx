import LegalDocument from '../components/LegalDocument';
import { cookieSections } from '../lib/legal';

export default function Cookies() { return <LegalDocument title="Cookie and Browser Storage Notice" updated="2026-10-05" sections={cookieSections} />; }
