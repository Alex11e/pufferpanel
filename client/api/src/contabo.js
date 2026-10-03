export class ContaboApi {
  _api = null

  constructor(api) {
    this._api = api
  }

  async instances() {
    return (await this._api.get('/api/contabo/instances')).data
  }

  async instance(id) {
    return (await this._api.get(`/api/contabo/instances/${id}`)).data
  }

  async action(id, action) {
    return (await this._api.post(`/api/contabo/instances/${id}/actions/${action}`)).data
  }

  async reinstall(id, data) {
    return (await this._api.put(`/api/contabo/instances/${id}/reinstall`, data)).data
  }

  async enableFirewallAddon(id) {
    return (await this._api.post(`/api/contabo/instances/${id}/firewall-addon`, { confirmation: 'BUY FIREWALL ADD-ON' })).data
  }

  async images() {
    return (await this._api.get('/api/contabo/images')).data
  }

  async createImage(data) {
    return (await this._api.post('/api/contabo/images', data)).data
  }

  async firewalls() {
    return (await this._api.get('/api/contabo/firewalls')).data
  }

  async createFirewall(data) {
    return (await this._api.post('/api/contabo/firewalls', data)).data
  }

  async updateFirewallRules(id, rules) {
    return (await this._api.put(`/api/contabo/firewalls/${id}/rules`, { rules })).data
  }

  async assignFirewall(id, instanceId) {
    return (await this._api.post(`/api/contabo/firewalls/${id}/instances/${instanceId}`)).data
  }

  async unassignFirewall(id, instanceId) {
    return (await this._api.delete(`/api/contabo/firewalls/${id}/instances/${instanceId}`)).data
  }
}