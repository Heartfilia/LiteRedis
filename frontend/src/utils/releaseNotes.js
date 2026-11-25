import { PENDING_RELEASE_NOTES, RELEASE_NOTES } from '../changelog.js'

const RELEASE_URL = 'https://github.com/Heartfilia/LiteRedis/releases/tag/v'

function versionLabel(version) {
  return String(version ?? '').trim()
}

function isLocalVersion(version) {
  return !version || /^dev$/i.test(version) || /-local(?:$|[.+-])/i.test(version)
}

function releaseKey(version) {
  return version.replace(/^v/i, '')
}

function translatedBody(notes, language) {
  const locale = String(language).toLowerCase().split('-')[0]
  // Historical strings are Chinese only; never pass them off as translations.
  const body = typeof notes === 'string' ? (locale === 'zh' ? notes : null) : notes?.[locale]
  return typeof body === 'string' && body.trim() ? body : null
}

/** Look up this exact release, or preview unreleased notes for dev/local builds. */
export function getLocalReleaseNotes(version, language = 'zh') {
  const label = versionLabel(version)
  const local = isLocalVersion(label)
  const key = releaseKey(label)
  const notes = local ? PENDING_RELEASE_NOTES : Object.hasOwn(RELEASE_NOTES, key) ? RELEASE_NOTES[key] : null
  const body = translatedBody(notes, language)
  if (!body) return null
  return {
    version: local ? label || 'dev' : key,
    body,
    url: local ? '' : RELEASE_URL + key,
  }
}

/** CI gate: true only when an exact published entry has nonempty zh and en bodies. */
export function validateReleaseNotes(version) {
  const label = versionLabel(version)
  if (isLocalVersion(label)) return false
  const key = releaseKey(label)
  if (!Object.hasOwn(RELEASE_NOTES, key)) return false
  const notes = RELEASE_NOTES[key]
  return (
    typeof notes === 'object' &&
    notes !== null &&
    typeof notes.zh === 'string' &&
    notes.zh.trim().length > 0 &&
    typeof notes.en === 'string' &&
    notes.en.trim().length > 0
  )
}
