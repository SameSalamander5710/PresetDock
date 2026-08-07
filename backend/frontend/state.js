// --------------------------------------------------------------------------
// Shared state and mutation helpers
// --------------------------------------------------------------------------

// Data caches
let presetsCache = [];
let decksCache = [];
let favouritesCache = [];

// View mode state
let viewMode = 'single'; // 'single' | 'dual'

// Pane instances
let leftPane = null;
let rightPane = null;

// Card actions reference (shared across pane renders)
let cardActionsRef = null;

// --------------------------------------------------------------------------
// Mutation helpers
// --------------------------------------------------------------------------

function setPresets(presets) {
  presetsCache = presets;
}

function setDecks(decks) {
  decksCache = decks;
}

function setFavourites(favourites) {
  favouritesCache = favourites;
}

function setViewModeState(mode) {
  viewMode = mode;
}

function setLeftPane(pane) {
  leftPane = pane;
}

function setRightPane(pane) {
  rightPane = pane;
}

function setCardActionsRef(ref) {
  cardActionsRef = ref;
}

function getCardActionsRef() {
  return cardActionsRef;
}

// --------------------------------------------------------------------------
// Pane state factory
// --------------------------------------------------------------------------

function createPaneState(paneElement) {
  const searchInput = paneElement.querySelector('.pane-search-input');
  const searchClear = paneElement.querySelector('.pane-search-clear');
  const suggestionsContainer = paneElement.querySelector('.pane-search-suggestions');
  const filterChipsContainer = paneElement.querySelector('.pane-filter-chips');
  const deckSelector = paneElement.querySelector('.pane-deck-selector');
  const presetsContainer = paneElement.querySelector('.pane-presets');
  const filterCountSpan = paneElement.querySelector('.pane-filter-count');

  return {
    el: paneElement,
    searchInput,
    searchClear,
    suggestionsContainer,
    filterChipsContainer,
    deckSelector,
    presetsContainer,
    filterCountSpan,
    searchText: '',
    activeFilters: [], // [{type: 'tag'|'engine', value: string}]
    activeDeckFilter: '',
    suggestionHighlightIndex: -1,
    currentSuggestions: [],
  };
}