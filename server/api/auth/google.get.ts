import { createOAuthState, googleOAuth } from '../../utils/oauth'

export default defineEventHandler((event) => {
  const state = createOAuthState(event)
  return sendRedirect(event, googleOAuth.buildAuthUrl(state))
})
