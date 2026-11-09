import {register} from './core.js';
import tokens from './tokens.js';

function all(root, selector) {
  const matches = [...root.querySelectorAll(selector)];
  if (root.nodeType === Node.ELEMENT_NODE && root.matches(selector)) matches.unshift(root);
  return matches;
}

function emit(element, type = 'change') {
  element.dispatchEvent(new Event(type, {bubbles: true}));
}

function number(value, fallback = 0) {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function bounded(value, min, max) {
  return Math.min(max, Math.max(min, value));
}

function setGlyph(button, name) {
  const glyph = button.querySelector('.l-glyph');
  if (glyph) glyph.textContent = name;
}

function syncSearch(search) {
  const input = search.querySelector('input');
  const clear = search.querySelector('[data-l-search-clear]');
  if (!input) return;
  const hasValue = input.value.length > 0;
  search.toggleAttribute('data-l-has-value', hasValue);
  if (clear) clear.hidden = !hasValue;
}

function syncSlider(slider) {
  const field = slider.closest('.l-slider-field');
  if (!field) return;
  const min = number(slider.min, 0);
  const max = number(slider.max, 100);
  if (!slider.hasAttribute('step')) {
    const keyboardStep = (max - min) * tokens.size.sliderKeyboardStep;
    if (keyboardStep > 0) slider.step = String(keyboardStep);
  }
  const value = bounded(number(slider.value, min), min, max);
  const fraction = max === min ? 0 : (value - min) / (max - min);
  field.style.setProperty('--l-value', String(fraction));
  slider.setAttribute('aria-valuetext', slider.dataset.lValueText || String(value));
  const output = field.querySelector('[data-l-slider-value]');
  if (output) output.textContent = slider.dataset.lValueText || String(value);
}

function stepperParts(stepper) {
  return {
    value: stepper.querySelector('.l-stepper-value'),
    decrement: stepper.querySelector('[data-l-stepper="decrement"]'),
    increment: stepper.querySelector('[data-l-stepper="increment"]'),
  };
}

function syncStepper(stepper, nextValue = number(stepper.dataset.value, 0), notify = false) {
  const min = number(stepper.dataset.min, Number.NEGATIVE_INFINITY);
  const max = number(stepper.dataset.max, Number.POSITIVE_INFINITY);
  const value = bounded(nextValue, min, max);
  const parts = stepperParts(stepper);
  stepper.dataset.value = String(value);
  if (parts.value) {
    parts.value.value = String(value);
    parts.value.textContent = String(value);
    parts.value.setAttribute('aria-valuenow', String(value));
    if (Number.isFinite(min)) parts.value.setAttribute('aria-valuemin', String(min));
    if (Number.isFinite(max)) parts.value.setAttribute('aria-valuemax', String(max));
  }
  if (parts.decrement) {
    parts.decrement.disabled = value <= min;
    parts.decrement.setAttribute('aria-disabled', String(value <= min));
  }
  if (parts.increment) {
    parts.increment.disabled = value >= max;
    parts.increment.setAttribute('aria-disabled', String(value >= max));
  }
  if (notify) {
    emit(stepper, 'input');
    emit(stepper, 'change');
  }
}

function moveStepper(stepper, direction) {
  const step = number(stepper.dataset.step, 1);
  syncStepper(stepper, number(stepper.dataset.value, 0) + direction * step, true);
}

function selectedButton(group, selector, attribute) {
  return group.querySelector(`${selector}[${attribute}="true"]`) || group.querySelector(selector);
}

function syncGlider(group, selector, attribute) {
  const selected = selectedButton(group, selector, attribute);
  if (!selected) return;
  const groupRect = group.getBoundingClientRect();
  const selectedRect = selected.getBoundingClientRect();
  const x = selectedRect.left - groupRect.left + group.scrollLeft;
  const oldX = group.style.getPropertyValue('--l-active-x');
  const oldWidth = group.style.getPropertyValue('--l-active-width');
  const nextX = `${x}px`;
  const nextWidth = `${selectedRect.width}px`;
  if (oldX !== nextX) group.style.setProperty('--l-active-x', nextX);
  if (oldWidth !== nextWidth) group.style.setProperty('--l-active-width', nextWidth);
}

function selectFilter(group, button, notify = true) {
  const buttons = [...group.querySelectorAll('.l-filter')];
  buttons.forEach(candidate => {
    const selected = candidate === button;
    candidate.setAttribute('aria-pressed', String(selected));
    candidate.tabIndex = selected ? 0 : -1;
  });
  group.dataset.value = button.dataset.value || button.textContent.trim();
  syncGlider(group, '.l-filter', 'aria-pressed');
  if (notify) {
    emit(group, 'input');
    emit(group, 'change');
  }
}

function panelFor(root, tab) {
  const id = tab.getAttribute('aria-controls');
  if (!id) return null;
  const escaped = CSS.escape(id);
  return root.querySelector(`#${escaped}`);
}

function selectTab(root, list, tab, notify = true) {
  const tabs = [...list.querySelectorAll('[role="tab"]')];
  tabs.forEach(candidate => {
    const selected = candidate === tab;
    candidate.setAttribute('aria-selected', String(selected));
    candidate.tabIndex = selected ? 0 : -1;
    const panel = panelFor(root, candidate);
    if (panel) panel.hidden = !selected;
  });
  list.dataset.value = tab.dataset.value || tab.textContent.trim();
  syncGlider(list, '.l-tab', 'aria-selected');
  if (notify) {
    emit(list, 'input');
    emit(list, 'change');
  }
}

function setDisclosure(disclosure, open, notify = true) {
  const trigger = disclosure.querySelector('[data-l-disclosure-trigger]');
  const content = disclosure.querySelector('.l-disclosure-content');
  if (!trigger || !content) return;
  disclosure.toggleAttribute('data-l-open', open);
  trigger.setAttribute('aria-expanded', String(open));
  content.setAttribute('aria-hidden', String(!open));
  content.inert = !open;
  disclosure.style.setProperty('--l-disclosure-height', open ? `${content.scrollHeight}px` : '0px');
  if (notify) emit(disclosure, 'change');
}

function adjacent(buttons, current, key) {
  let index = buttons.indexOf(current);
  if (key === 'Home') index = 0;
  else if (key === 'End') index = buttons.length - 1;
  else if (key === 'ArrowLeft') index = (index - 1 + buttons.length) % buttons.length;
  else if (key === 'ArrowRight') index = (index + 1) % buttons.length;
  return buttons[index];
}

register('controls', root => {
  all(root, '.l-search').forEach(syncSearch);
  all(root, '.l-slider').forEach(syncSlider);
  all(root, '.l-stepper').forEach(stepper => syncStepper(stepper));
  all(root, '.l-filters').forEach(group => {
    const initial = selectedButton(group, '.l-filter', 'aria-pressed');
    if (initial) selectFilter(group, initial, false);
  });
  all(root, '.l-tabs[role="tablist"]').forEach(list => {
    const initial = selectedButton(list, '[role="tab"]', 'aria-selected');
    if (initial) selectTab(root, list, initial, false);
  });
  all(root, '.l-disclosure').forEach(disclosure => {
    const trigger = disclosure.querySelector('[data-l-disclosure-trigger]');
    setDisclosure(disclosure, trigger?.getAttribute('aria-expanded') === 'true', false);
  });

  root.addEventListener('click', event => {
    const blocked = event.target.closest(':is(.l-plaque, .l-icon-plaque):is([aria-busy="true"], [aria-disabled="true"])');
    if (!blocked) return;
    event.preventDefault();
    event.stopImmediatePropagation();
  }, true);

  root.addEventListener('click', event => {
    const reveal = event.target.closest('[data-l-reveal]');
    if (reveal) {
      const field = reveal.closest('.l-field-control')?.querySelector('.l-text-field');
      if (!field) return;
      const showing = field.type === 'text';
      field.type = showing ? 'password' : 'text';
      reveal.setAttribute('aria-pressed', String(!showing));
      reveal.setAttribute('aria-label', showing ? 'Show value' : 'Hide value');
      setGlyph(reveal, showing ? 'visibility' : 'visibility_off');
      field.focus();
      emit(field, 'change');
      return;
    }

    const clear = event.target.closest('[data-l-search-clear]');
    if (clear) {
      const search = clear.closest('.l-search');
      const input = search?.querySelector('input');
      if (!input) return;
      input.value = '';
      syncSearch(search);
      input.focus();
      emit(input, 'input');
      emit(input, 'change');
      return;
    }

    const toggle = event.target.closest('.l-toggle[role="switch"]');
    if (toggle) {
      if (toggle.matches(':disabled, [aria-disabled="true"]')) return;
      toggle.setAttribute('aria-checked', String(toggle.getAttribute('aria-checked') !== 'true'));
      emit(toggle, 'input');
      emit(toggle, 'change');
      return;
    }

    const stepButton = event.target.closest('[data-l-stepper]');
    if (stepButton) {
      const stepper = stepButton.closest('.l-stepper');
      if (!stepper || stepButton.disabled) return;
      moveStepper(stepper, stepButton.dataset.lStepper === 'increment' ? 1 : -1);
      return;
    }

    const filter = event.target.closest('.l-filter');
    if (filter) {
      const group = filter.closest('.l-filters');
      if (group) selectFilter(group, filter);
      return;
    }

    const tab = event.target.closest('[role="tab"].l-tab');
    if (tab) {
      const list = tab.closest('.l-tabs[role="tablist"]');
      if (list) selectTab(root, list, tab);
      return;
    }

    const more = event.target.closest('[data-l-split-more]');
    if (more) {
      const split = more.closest('.l-split-plaque');
      const open = more.getAttribute('aria-expanded') !== 'true';
      more.setAttribute('aria-expanded', String(open));
      setGlyph(more, open ? 'expand_less' : 'expand_more');
      if (split) emit(split, 'change');
      return;
    }

    const disclosureTrigger = event.target.closest('[data-l-disclosure-trigger]');
    if (disclosureTrigger) {
      const disclosure = disclosureTrigger.closest('.l-disclosure');
      if (disclosure) setDisclosure(disclosure, disclosureTrigger.getAttribute('aria-expanded') !== 'true');
    }
  });

  root.addEventListener('input', event => {
    if (event.target.matches('.l-slider')) syncSlider(event.target);
    if (event.target.matches('.l-search input')) syncSearch(event.target.closest('.l-search'));
  });

  root.addEventListener('keydown', event => {
    if (event.key === 'Escape' && event.target.matches('.l-search input')) {
      const search = event.target.closest('.l-search');
      if (!event.target.value) return;
      event.target.value = '';
      syncSearch(search);
      emit(event.target, 'input');
      emit(event.target, 'change');
      return;
    }

    const stepperValue = event.target.closest('.l-stepper-value');
    if (stepperValue && ['ArrowLeft', 'ArrowDown', 'ArrowRight', 'ArrowUp', 'Home', 'End'].includes(event.key)) {
      const stepper = stepperValue.closest('.l-stepper');
      event.preventDefault();
      if (event.key === 'Home') syncStepper(stepper, number(stepper.dataset.min, 0), true);
      else if (event.key === 'End') syncStepper(stepper, number(stepper.dataset.max, 0), true);
      else moveStepper(stepper, ['ArrowRight', 'ArrowUp'].includes(event.key) ? 1 : -1);
      return;
    }

    const filter = event.target.closest('.l-filter');
    if (filter && ['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) {
      const group = filter.closest('.l-filters');
      const buttons = [...group.querySelectorAll('.l-filter:not(:disabled)')];
      const next = adjacent(buttons, filter, event.key);
      event.preventDefault();
      next.focus();
      selectFilter(group, next);
      return;
    }

    const tab = event.target.closest('[role="tab"].l-tab');
    if (tab && ['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) {
      const list = tab.closest('.l-tabs[role="tablist"]');
      const tabs = [...list.querySelectorAll('[role="tab"]:not(:disabled)')];
      const next = adjacent(tabs, tab, event.key);
      event.preventDefault();
      next.focus();
      selectTab(root, list, next);
    }
  });

  const documentRoot = root.nodeType === Node.DOCUMENT_NODE ? root : root.ownerDocument;
  documentRoot.defaultView?.addEventListener('resize', () => {
    all(root, '.l-filters').forEach(group => syncGlider(group, '.l-filter', 'aria-pressed'));
    all(root, '.l-tabs[role="tablist"]').forEach(list => syncGlider(list, '.l-tab', 'aria-selected'));
    all(root, '.l-disclosure[data-l-open]').forEach(disclosure => setDisclosure(disclosure, true, false));
  });
});
