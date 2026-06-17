import {
  AppWrapperRoute,
  defineWebApplication
} from '@opencloud-eu/web-pkg'
import AktenplanView from './views/AktenplanView.vue'
import AkteView from './views/AkteView.vue'
import VorgangView from './views/VorgangView.vue'
import RegisterView from './views/RegisterView.vue'

const applicationId = 'folderviews'

export default defineWebApplication({
  setup() {
    const routes = [
      {
        name: 'folderviews',
        path: '/:driveAliasAndItem(.*)?',
        component: AppWrapperRoute(AktenplanView, { applicationId }),
        meta: { authContext: 'hybrid', patchCleanPath: true }
      }
    ]

    const appInfo = {
      name: 'Folder Views',
      id: applicationId,
      icon: 'archive',
      color: '#5c6bc0',
      defaultExtension: '',
      extensions: [] as any[]
    }

    // FolderView handlers keyed by .type_ name
    const folderViewHandlers = {
      aktenplan: AktenplanView,
      akte: AkteView,
      vorgang: VorgangView,
      register: RegisterView
    }

    return {
      appInfo,
      routes,
      folderViewHandlers
    }
  }
})
