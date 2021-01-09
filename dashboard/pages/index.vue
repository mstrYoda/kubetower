<template>
  <div class="container">
    <div>
      <Logo />
      <h1 class="title">Kubetower dashboard</h1>
      <ul>
        <li
          v-for="(deployment, index) in deployments['kind-local']"
          v-key="index"
        >
          <span>{{ deployment.metadata.name }}</span>
          <span
            >Available replicas =
            {{ deployment.status.availableReplicas }}</span
          >
        </li>
      </ul>
    </div>
  </div>
</template>

<script>
import deployments from '~/mocks/deployments.json'
import { getClusters, getDeployments } from '~/apis'
const isLocal = true

export default {
  // https://nuxtjs.org/examples/data-fetching-async-data
  async asyncData() {
    let deploymentsData = deployments
    const clusters = getClusters()
    if (!isLocal) {
      const BASE_URL = 'http://localhost:8080'

      deploymentsData = await getDeployments(BASE_URL, clusters.join(','))
    }

    return { clusters, deployments: deploymentsData }
  },
}
</script>
