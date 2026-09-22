
import { EntityPage } from '../components/EntityPage';
import { ENTITY_CONFIGS } from '../types/status';
import { useSpecimenStore } from '../stores/specimen';
export default function SpecimenPage() { return <EntityPage config={ENTITY_CONFIGS[1]} useStore={useSpecimenStore} showRiskTags />; }
