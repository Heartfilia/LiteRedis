import { test } from 'node:test'
import assert from 'node:assert/strict'
import { registerEditorGuard, confirmDiscardEdits, hasUnsavedEdits } from './editSafety.js'
import { confirmation, resolveConfirmation, askConfirmation } from './confirm.js'

test('app-owned draft confirmation preserves cancelled drafts and blocks saving', async () => {
  let dirty = true
  let saving = false
  const unregister = registerEditorGuard(Symbol(), () => ({ dirty, saving, discard: () => { dirty = false } }))
  try {
    assert.equal(hasUnsavedEdits(), true)
    let pending = confirmDiscardEdits()
    assert.ok(confirmation.value)
    resolveConfirmation(false)
    assert.equal(await pending, false)
    assert.equal(dirty, true)
    pending = confirmDiscardEdits()
    resolveConfirmation(true)
    assert.equal(await pending, true)
    assert.equal(await confirmDiscardEdits(), true)
    saving = true
    pending = confirmDiscardEdits()
    resolveConfirmation(true)
    assert.equal(await pending, false)
  } finally { resolveConfirmation(false); unregister() }
  assert.equal(hasUnsavedEdits(), false)
})

test('overlapping confirmations are declined and saving is rechecked', async () => {
  let saving = false
  const unregister = registerEditorGuard(Symbol(), () => ({ dirty: true, saving }))
  try {
    const first = confirmDiscardEdits()
    assert.equal(await askConfirmation('overlap'), false)
    saving = true
    resolveConfirmation(true)
    assert.equal(await first, false)
  } finally { resolveConfirmation(false); unregister() }
})

test('a replaced editor cannot be discarded by an older confirmation', async () => {
  const id = Symbol()
  let discarded = false
  let unregister = registerEditorGuard(id, () => ({ dirty: true, discard: () => { discarded = true } }))
  try {
    const pending = confirmDiscardEdits()
    unregister()
    unregister = registerEditorGuard(id, () => ({ dirty: true }))
    resolveConfirmation(true)
    assert.equal(await pending, false)
    assert.equal(discarded, false)
  } finally { resolveConfirmation(false); unregister() }
})
