import landing from './landing'
import common from './common'
import dashboard from './dashboard'
import admin from './admin'
import misc from './misc'
import modelMarket from './modelMarket'
import developers from './developers'
import channelStatus from './channelStatus'

export default {
  ...landing,
  ...common,
  ...dashboard,
  ...modelMarket,
  ...developers,
  ...channelStatus,
  admin,
  ...misc,
}
