#!/usr/bin/env ruby
# frozen_string_literal: true

require 'net/http'
require 'openssl'
require 'uri'
require 'time'

# Securely sends an HTTP request with HMAC authentication headers.
# @param url_str [String] The full URL of the endpoint.
# @param method [String] HTTP method (GET, POST, UPDATE, etc.)
# @param body [String] The request body (default is empty string).
# @return [Net::HTTPResponse]
def secure_request(url_str, method, body = '')
  uri = URI.parse(url_str)
  secret_key = ENV['BWW_SECRET_KEY']
  
  if secret_key.nil? || secret_key.empty?
    warn "Error: BWW_SECRET_KEY environment variable is not set."
    exit 1
  end

  timestamp = Time.now.to_i.to_s
  path = uri.path
  
  # The signature is calculated over: Timestamp + Method + Path + Body
  message = timestamp + method.upcase + path + body
  
  # Create HMAC-SHA256 signature
  signature = OpenSSL::HMAC.hexdigest('SHA256', secret_key, message)

  # Prepare HTTP request
  http = Net::HTTP.new(uri.host, uri.port)
  
  # Handle custom method names like 'UPDATE'
  request = Net::HTTPGenericRequest.new(
    method.upcase,
    !body.empty?,
    true,
    uri.request_uri
  )
  
  # Set auth headers
  request['X-BWW-Timestamp'] = timestamp
  request['X-BWW-Signature'] = signature
  request.body = body unless body.empty?

  # Execute request
  http.request(request)
end

def main
  base_url = ENV['BWW_URL'] || 'http://localhost:8080'
  
  puts "BWW Client Example starting..."
  puts "Base URL: #{base_url}"

  puts "\n Triggering Sync..."
  res = secure_request("#{base_url}/sync", 'UPDATE')
  puts "Response Code: #{res.code}"
  puts "Response Body: #{res.body}"

  puts "\n Rendering Template..."
  template = "Password is: __{{koishi.litellm}}__\nField is: __{{infra.common-users:f:hh}}__\nFile is: __{{koishi.foldersync.sshkeys.system.files:a:ssh_host_ecdsa_key}}__\nFile Base64 is: __{{koishi.foldersync.sshkeys.system.files:a:ssh_host_ecdsa_key:a:b64}}__"
  res = secure_request("#{base_url}/render", 'POST', template)
  puts "Response Code: #{res.code}"
  puts "Response Body: #{res.body}"

  puts "\n Fetching Password ..."
  res = secure_request("#{base_url}/koishi.litellm/password", 'GET')
  puts "Response Code: #{res.code}"
  puts "Response Body: #{res.body}"

  puts "\n Fetching Field ..."
  res = secure_request("#{base_url}/infra.common-users/field/hh", 'GET')
  puts "Response Code: #{res.code}"
  puts "Response Body: #{res.body}"

  puts "\n Fetching File ..."
  res = secure_request("#{base_url}/koishi.foldersync.sshkeys.system.files/attachment/ssh_host_ecdsa_key", 'GET')
  puts "Response Code: #{res.code}"
  puts "Response Body: #{res.body}"

  puts "\n Fetching File with base64 ..."
  res = secure_request("#{base_url}/koishi.foldersync.sshkeys.system.files/attachment/ssh_host_ecdsa_key/base64", 'GET')
  puts "Response Code: #{res.code}"
  puts "Response Body: #{res.body}"
end

main if __FILE__ == $PROGRAM_NAME
