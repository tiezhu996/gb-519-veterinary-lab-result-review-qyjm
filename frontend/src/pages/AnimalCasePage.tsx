
import { EntityPage } from '../components/EntityPage';
import { ENTITY_CONFIGS } from '../types/status';
import { useAnimalCaseStore } from '../stores/animal-case';
export default function AnimalCasePage() { return <EntityPage config={ENTITY_CONFIGS[0]} useStore={useAnimalCaseStore} showRiskTags />; }
