import { validateReleaseNotes } from '../src/utils/releaseNotes.js'

const version = process.argv[2] || process.env.GITHUB_REF_NAME
if (!version || !validateReleaseNotes(version)) {
  console.error(`Missing Chinese/English release notes for ${version || '(no version)'}. Add the exact release version to src/changelog.js before publishing.`)
  process.exitCode = 1
} else {
  console.log(`Chinese/English release notes verified for ${version}`)
}
