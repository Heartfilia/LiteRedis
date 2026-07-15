function contextsEqual(left, right) {
  const leftKeys = Object.keys(left || {})
  const rightKeys = Object.keys(right || {})
  if (leftKeys.length !== rightKeys.length) return false
  return leftKeys.every(key => Object.is(left[key], right[key]))
}

export function createRequestGuard(getContext) {
  let epoch = 0
  let sequence = 0
  const latestByScope = new Map()

  function currentContext() {
    return { ...(getContext?.() || {}) }
  }

  return {
    begin(scope = 'default') {
      const token = {
        scope,
        epoch,
        sequence: ++sequence,
        context: currentContext(),
      }
      latestByScope.set(scope, token.sequence)
      return token
    },

    isCurrent(token) {
      if (!token || token.epoch !== epoch) return false
      if (latestByScope.get(token.scope) !== token.sequence) return false
      return contextsEqual(token.context, currentContext())
    },

    finish(token) {
      if (token && latestByScope.get(token.scope) === token.sequence) {
        latestByScope.delete(token.scope)
      }
    },

    invalidate(scope) {
      latestByScope.delete(scope)
    },

    invalidateAll() {
      epoch++
      latestByScope.clear()
    },
  }
}
