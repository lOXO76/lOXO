import landing from './landing'
import common from './common'
import enterprise from './enterprise'
import dashboard from './dashboard'
import channelMonitorV2 from './channelMonitorV2'
import batchImage from './batchImage'
import admin from './admin'
import misc from './misc'
import asyncTasks from './asyncTasks'
import apiDocs from './apiDocs'
import checkIn from './checkIn'
import clientImport from './clientImport'
import degradation from './degradation'
import creation from './creation'
import rewardTasks from './rewardTasks'
import rewardIntegration from './rewardIntegration'

export default {
  ...landing,
  ...common,
  ...enterprise,
  ...dashboard,
  ...channelMonitorV2,
  ...batchImage,
  ...misc,
  ...asyncTasks,
  ...apiDocs,
  ...checkIn,
  ...degradation,
  ...clientImport,
  ...creation,
  ...rewardTasks,
  ...rewardIntegration,
  admin: { ...admin, ...degradation.admin },
  nav: { ...common.nav, ...checkIn.nav, ...clientImport.nav, ...degradation.nav, ...creation.nav, ...rewardTasks.nav },
}
