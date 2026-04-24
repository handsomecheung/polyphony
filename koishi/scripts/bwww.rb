#!/usr/bin/env ruby
# frozen_string_literal: true

require 'net/http'
require 'uri'
require 'find'
require 'openssl'
require 'time'

$stdout.sync = true

module BWWW
  def self.get_config
    @config ||= begin
      url = ENV['BWW_URL']
      if url.nil? || url.empty?
        warn "Error: BWW_URL is not defined in env or is empty."
        exit 1
      end

      secret_key = ENV['BWW_SECRET_KEY']
      if secret_key.nil? || secret_key.empty?
        warn "Error: BWW_SECRET_KEY is not defined in env or is empty."
        exit 1
      end

      {
        'BWW_URL' => url,
        'BWW_SECRET_KEY' => secret_key,
      }
    end
  end

  def self.base_url
    @base_url ||= get_config['BWW_URL'].chomp('/')
  end

  def self.secret_key
    @secret_key ||= get_config['BWW_SECRET_KEY']
  end

  def self.get_password(name, base64 = false)
    path = "/#{name}/password"
    path += '/base64' if base64
    request_get(path)
  end

  def self.get_field(name, field)
    request_get("/#{name}/field/#{field}")
  end

  def self.get_attachment(name, attachment, base64 = false)
    path = "/#{name}/attachment/#{attachment}"
    path += '/base64' if base64
    request_get(path)
  end

  def self.download_attachment(name, attachment, output)
    content = get_attachment name, attachment, false
    File.write(output, content)
  end

  def self.sync
    uri = URI("#{base_url}/sync")
    res = secure_request(uri, 'UPDATE')

    unless res.is_a?(Net::HTTPSuccess)
      warn "Warning: Sync failed: #{res.code} #{res.message}"
      return nil
    end

    puts "BWWW Sync successful: #{res.body}"
  end

  def self.render_file(path)
    if File.directory?(path)
      render_for_dir(path)
    else
      render_for_file(path)
    end
  end

  def self.render_content(content)
    filepath = File.join gen_tmpdir, 'tmpfile_for_render'
    File.write filepath, content
    get_render_content filepath
  end

  private

  def self.gen_tmpdir
    tempdir = Dir.mktmpdir('bwww-', '/tmp')
    at_exit { FileUtils.remove_entry(tempdir) }

    tempdir
  end

  def self.render_for_file(path)
    puts "Render file: #{path}"

    content = get_render_content(path)
    if content
      File.write(path, content)
    else
      puts "Failed to Render file #{path}"
      exit 1
    end
  end
def self.render_for_dir(path)
  Find.find(path) do |p|
    if File.directory?(p) && File.basename(p) == '.git'
      Find.prune
    end
    next if File.directory?(p)
    next unless text_file?(p)

    render_for_file(p)
  end
end

def self.text_file?(path)
  File.open(path, 'rb') do |f|
    chunk = f.read(1024)
    return true if chunk.nil? # empty file

    !chunk.include?("\0")
  end
rescue StandardError
  false
end

private
  def self.get_render_content(filepath)
    uri = URI("#{base_url}/render")
    content = File.read(filepath)
    res = secure_request(uri, 'POST', content)

    unless res.is_a?(Net::HTTPSuccess)
      warn "Warning: Failed to render #{filepath}: #{res.code} #{res.message}"
      return nil
    end

    res.body
  end

  def self.request_get(path)
    uri = URI("#{base_url}#{path}")
    res = secure_request(uri, 'GET')
    
    unless res.is_a?(Net::HTTPSuccess)
      warn "Warning: Request to #{uri} failed: #{res.code} #{res.message}"
      return nil
    end
    
    res.body
  end

  def self.secure_request(uri, method, body = '')
    key = secret_key
    if key.nil? || key.empty?
      warn "Error: BWW_SECRET_KEY is not defined in bwww/config.rb."
      exit 1
    end

    timestamp = Time.now.to_i.to_s
    message = timestamp + method.upcase + uri.path + body
    signature = OpenSSL::HMAC.hexdigest('SHA256', key, message)

    http = Net::HTTP.new(uri.hostname, uri.port)
    http.use_ssl = (uri.scheme == 'https')
    http.open_timeout = 300
    http.read_timeout = 300
    http.write_timeout = 300

    req = Net::HTTPGenericRequest.new(
      method.upcase,
      !body.empty?,
      true,
      uri.request_uri
    )
    req['X-BWW-Timestamp'] = timestamp
    req['X-BWW-Signature'] = signature
    req.body = body unless body.empty?
    req.content_type = 'text/plain' if method.upcase == 'POST'

    http.request(req)
  end
end

def main
  if ARGV.empty?
    return
  end

  method = ARGV[0]
  case method
  when 'get-password'
    puts BWWW.get_password(ARGV[1], ARGV[2] == 'base64')
  when 'get-field'
    puts BWWW.get_field(ARGV[1], ARGV[2])
  when 'get-attachment'
    print BWWW.get_attachment(ARGV[1], ARGV[2], ARGV[3] == 'base64')
  when 'download-attachment'
    print BWWW.download_attachment(ARGV[1], ARGV[2], ARGV[3])
  when 'render-file'
    BWWW.render_file(ARGV[1])
  when 'render-content'
    BWWW.render_content(ARGV[1])
  when 'sync'
    BWWW.sync
  else
    warn "invalid method #{method}"
    exit 1
  end
end

main if __FILE__ == $PROGRAM_NAME
