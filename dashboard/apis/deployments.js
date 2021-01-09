export const getDeployments = (BASE_URL, clusters = 'kind-local') => {
  return fetch(
    `${BASE_URL}/resources/deployments?clusters=${clusters}`
  ).then((res) => res.json())
}
