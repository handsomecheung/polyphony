# frozen_string_literal: true

require 'yaml'
require 'tmpdir'
require File.join(File.expand_path(__dir__), 'common')
require File.join('~/scripts/workspace/my-secret.scripts/lib', 'bw')

CMD_KUBECTL = "kubectl"

CLOUDPRIVATE_REGISTRY_PREFIX = 'cloudprivate/'
CLOUDPUBLIC_REGISTRY_PREFIX = 'cloudpublic/'
CLOUDPRIVATE_REGISTRY_SECRET_NAME = 'dockersecret-cloudprivate'

module Kubectl
  def self.get_secret_config(key)
    BW.get_password_by_key "koishi.deploy.#{key}"
  end

  def self.get_timezone
    @timezone ||= get_secret_config 'timezone'
  end

  def self.get_cloudprivate_registry_id
    @cloudprivate_registry_id ||= get_secret_config 'cloudprivate_registry_id'
  end

  def self.get_cloudprivate_registry_host
    @cloudprivate_registry_host ||= get_secret_config 'cloudprivate_registry_host'
  end

  def self.get_cloudpublic_registry_id
    @cloudpublic_registry_id ||= get_secret_config 'cloudpublic_registry_id'
  end

  def self.get_cloudpublic_registry_host
    @cloudpublic_registry_host ||= get_secret_config 'cloudpublic_registry_host'
  end

  def self.gen_tmpdir
    tempdir = Dir.mktmpdir('my-k8s-deploy-', '/tmp')
    at_exit { FileUtils.remove_entry(tempdir) }

    tempdir
  end

  def self.setup_config(raw_config_file, options)
    docs = rewrite_config raw_config_file, options
    deploy docs, options
  end

  def self.setup_meta(meta, options)
    unless meta == 'pullsecret-cloudprivate'
      puts 'unknown meta, only support pullsecret-cloudprivate'
      exit 1
    end

    namespace = options['namespace']
    if namespace.nil?
      puts 'namespace is required'
      exit 1
    end

    doc = create_doc_pullsecret_cloudprivate namespace
    deploy [doc], options
  end

  def self.rewrite_config(raw_config_file, options)
    c = config_replace_placeholder(raw_config_file, options)
    config_add_predefinition config_replace_secret(c)
  end

  def self.config_replace_placeholder(raw_config_file, options)
    holders = options.select { |k, _v| k.start_with? 'var.' }.transform_keys { |k| k.sub('var.', '') }

    content = File.read(raw_config_file)
    holders.each_pair do |k, v|
      content = content.gsub("__((#{k}))__", v)
    end

    content
  end

  def self.bw_render_text(content)
    value, errors = BW.render content
    unless errors.empty?
      puts errors
      exit 1
    end

    value
  end

  def self.bw_download_attachment(secret_name, attachment_name)
    filepath = File.join gen_tmpdir, attachment_name

    error = BW.download_attachment secret_name, attachment_name, filepath
    unless error.nil?
      puts error
      exit 1
    end

    filepath
  end

  def self.config_replace_secret(content)
    bw_render_text content
  end

  def self.write_docs_to_file(docs)
    config_file = File.join gen_tmpdir, 'manifest.yaml'
    File.write(config_file, YAML.dump_stream(*docs))
    puts "\n\ngenerated config file: #{config_file}\n"

    config_file
  end

  def self.config_add_predefinition(content)
    docs = []
    YAML.load_stream(content) do |doc|
      case doc['kind']
      when 'Deployment'
        docs.concat config_add_predefinition_deployment(doc)
      when 'CronJob'
        docs.concat config_add_predefinition_cronjob(doc)
      when 'Job'
        docs.concat config_add_predefinition_job(doc)
      else
        docs << doc
      end
    end

    docs
  end

  def self.config_add_predefinition_spec!(spec)
    spec['terminationGracePeriodSeconds'] = 5 if spec['terminationGracePeriodSeconds'].nil?

    spec['containers'].each do |container|
      container['env'] ||= []
      container['env'] << { 'name' => 'TZ', 'value' => get_timezone } if container['env'].none? { |env| env['name'] == 'TZ' }
    end

    spec['nodeSelector'] ||= {}
    return unless spec['nodeSelector'].empty?

    spec['nodeSelector']['kubernetes.io/arch'] = 'amd64' unless check_available_arm_images(spec['containers'].map { |c| c['image'] })
  end

  def self.config_add_predefinition_deployment(doc)
    if doc['spec']['strategy'].nil?
      doc['spec']['strategy'] = {
        'type' => 'RollingUpdate',
        'rollingUpdate' => {
          'maxSurge' => '100%',
          'maxUnavailable' => 0,
        },
      }
    end

    spec = doc['spec']['template']['spec']

    config_add_predefinition_spec! spec
    pullsecret_docs = config_predefinition_registry get_namespace(doc), spec
    [doc] + pullsecret_docs
  end

  def self.config_add_predefinition_cronjob(doc)
    doc['spec']['timeZone'] = get_timezone if doc['spec']['timeZone'].nil?

    spec = doc['spec']['jobTemplate']['spec']['template']['spec']

    config_add_predefinition_spec! spec
    pullsecret_docs = config_predefinition_registry get_namespace(doc), spec
    [doc] + pullsecret_docs
  end

  def self.config_add_predefinition_job(doc)
    spec = doc['spec']['template']['spec']

    config_add_predefinition_spec! spec
    pullsecret_docs = config_predefinition_registry get_namespace(doc), spec
    [doc] + pullsecret_docs
  end

  def self.config_predefinition_registry(namespace, spec)
    (spec['containers'].to_a + spec['initContainers'].to_a).each do |container|
      if container['image'].start_with? CLOUDPRIVATE_REGISTRY_PREFIX
        container['image'] = container['image'].sub CLOUDPRIVATE_REGISTRY_PREFIX, "#{get_cloudprivate_registry_host}/#{get_cloudprivate_registry_id}/docker/"
        try_add_pullsecret spec, CLOUDPRIVATE_REGISTRY_SECRET_NAME
      elsif container['image'].start_with? CLOUDPUBLIC_REGISTRY_PREFIX
        container['image'] = container['image'].sub CLOUDPUBLIC_REGISTRY_PREFIX, "#{get_cloudpublic_registry_host}/#{get_cloudpublic_registry_id}/"
      end
    end

    try_create_pullsecret_docs namespace, spec
  end

  def self.try_create_pullsecret_docs(namespace, spec)
    docs = []
    spec['imagePullSecrets'].to_a.each do |secret|
      if secret['name'] == CLOUDPRIVATE_REGISTRY_SECRET_NAME
        docs << create_doc_pullsecret_cloudprivate(namespace)
      end
    end

    docs
  end

  def self.try_add_pullsecret(spec, name)
    spec['imagePullSecrets'] ||= []
    return unless spec['imagePullSecrets'].none? { |s| s['name'] == name }

    spec['imagePullSecrets'].append({ 'name' => name })
  end

  def self.check_available_arm_images(images)
    images.all? { |image| check_available_arm_image image }
  end

  def self.check_available_arm_image(image)
    archs = get_image_archs
    puts "\n\nimage #{image} supports architectures: #{archs.join(', ')}"
    archs.include? 'amd64'
  end

  def self.get_image_archs()
    return %w[amd64 arm64]
  end

  def self.run_command(command)
    code, out = Common.run_command command
    if code != 0
      puts "\nfailed to run command. code #{code}"
      exit 1
    end

    out
  end

  def self.create_doc_pullsecret_cloudprivate(namespace)
    key_file = bw_download_attachment 'gcp.files', 'pull-image.json'
    email = get_secret_config "email"
    out = run_command %{#{CMD_KUBECTL} -n #{namespace} create secret docker-registry #{CLOUDPRIVATE_REGISTRY_SECRET_NAME} --docker-server #{get_cloudprivate_registry_host}  --docker-username _json_key --docker-email #{email} --docker-password="$(cat #{key_file})" --dry-run=client  -o yaml}
    YAML.safe_load out
  end

  def self.deploy(docs, options)
    config_file = write_docs_to_file docs

    result = run_command "#{CMD_KUBECTL} apply -f #{config_file}"
    puts "temporaryarly skip restarting. result: \n#{result}"

    # try_restart config_file, result
    check_status config_file
  end

  def self.get_namespace(doc)
    namespace = doc['metadata']['namespace'].to_s
    namespace = 'default' if namespace.empty?
    namespace
  end

  def self.try_restart(config_file, deploy_result)
    YAML.load_stream(File.read(config_file)) do |doc|
      case doc['kind']
      when 'Deployment'
        name = doc['metadata']['name']
        status = deploy_result.split("\n").find { |line| line.include? "deployment.apps/#{name}" }.split(' ')[-1].strip
        restart_deployment get_namespace(doc), name if status == 'unchanged'
      end
    end
  end

  def self.restart_deployment(namespace, name)
    run_command "#{CMD_KUBECTL} -n #{namespace} rollout restart deployment #{name}"
  end

  def self.check_status(config_file)
    YAML.load_stream(File.read(config_file)) do |doc|
      case doc['kind']
      when 'Deployment'
        check_status_deployment get_namespace(doc), doc['metadata']['name']
      end
    end
  end

  def self.check_job_condition(namespace, name, condition)
    Common.run_command("#{CMD_KUBECTL} -n #{namespace} wait --for=condition=#{condition} --timeout=0 job #{name}", parse_json: false, raise_error: false)[0] == 0
  end

  def self.check_status_deployment(namespace, name)
    puts "\n\nwait for deployment #{name} in namespace #{namespace} to be done ..."
    status = Common.run_raw_shell "#{CMD_KUBECTL} -n #{namespace} rollout status deployment #{name} --timeout=300s", false

    return if status

    puts "\n\n\n"
    puts 'Timed out. Maybe deployment failed.'
    raise 'failed to check deployment status'
  end

  def self.print_logs(namespace, pod_name)
    puts "\n\nlogs:"
    _code, logs = Common.run_command "#{CMD_KUBECTL} -n #{namespace} logs #{pod_name}"
    puts logs
  end

  def self.list_cronjobs(region, selector = '')
    selector_arg = if selector.empty?
                     ''
                   else
                     "--selector #{selector}"
                   end
    data = Common.run_shell get_remote_cmd(region, "#{CMD_KUBECTL} get cronjobs #{selector_arg} -o json")
    data['items']
  end

  def self.delete_cronjob(region, name)
    Common.run_shell get_remote_cmd(region, "#{CMD_KUBECTL} delete cronjob #{name}"), false
  end

  def self.list_vpas(region)
    data = Common.run_shell get_remote_cmd(region, "#{CMD_KUBECTL} get vpa -o json")
    data['items']
  end

  def self.delete_vpa(region, name)
    Common.run_shell get_remote_cmd(region, "#{CMD_KUBECTL} delete vpa #{name}"), false
  end

  def self.list_deployments(region)
    data = Common.run_shell get_remote_cmd(region, "#{CMD_KUBECTL} get deployments -o json")
    data['items']
  end

  def self.get_deployment(service, region)
    Common.run_shell get_remote_cmd(region, "#{CMD_KUBECTL} get deployment #{service} -o json")
  end

  def self.set_image(kind, region, name, contianer, image)
    Common.run_raw_shell get_remote_cmd(region, "#{CMD_KUBECTL} set image #{kind}/#{name} #{contianer}=#{image}")
  end

  def self.set_deployment_image(region, name, contianer, image)
    set_image 'deployment', region, name, contianer, image
  end

  def self.set_cronjob_image(region, name, contianer, image)
    set_image 'cronjob', region, name, contianer, image
  end
end
