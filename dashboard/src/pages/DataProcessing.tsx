import LegalDocument from '../components/LegalDocument';
import { dpaSections } from '../lib/legal';

export default function DataProcessing() { return <LegalDocument title="Data Processing Addendum" updated="2026-10-05" sections={dpaSections} />; }
