// Initialize Lucide icons
lucide.createIcons();

// Constants
const TIME_MS = 500;
const SERVO_MIN = 0;
const SERVO_MAX = 180;
const DEFAULT_X = 10;
const DEFAULT_Y = 10;

// Application State
let state = {
  x: DEFAULT_X,
  y: DEFAULT_Y,
  stepSize: 5,
  isSimulation: false,
  isConnecting: true,
  lastSentX: DEFAULT_X,
  lastSentY: DEFAULT_Y
};

// DOM Elements
const elements = {
  connectionStatus: document.getElementById('connection-status'),
  simulationToggle: document.getElementById('simulation-toggle'),
  cameraStream: document.getElementById('camera-stream'),
  simulationCanvas: document.getElementById('simulation-canvas'),
  streamOverlayError: document.getElementById('stream-overlay-error'),
  reconnectBtn: document.getElementById('reconnect-btn'),
  refreshStreamBtn: document.getElementById('refresh-stream-btn'),
  fullscreenBtn: document.getElementById('fullscreen-btn'),
  streamTypeLabel: document.getElementById('stream-type-label'),
  streamSourceLabel: document.getElementById('stream-source-label'),
  
  valX: document.getElementById('val-x'),
  valY: document.getElementById('val-y'),
  stepSizeSelect: document.getElementById('step-size'),
  resetBtn: document.getElementById('reset-btn'),
  
  // D-Pad
  btnUp: document.getElementById('btn-up'),
  btnDown: document.getElementById('btn-down'),
  btnLeft: document.getElementById('btn-left'),
  btnRight: document.getElementById('btn-right'),
  btnCenter: document.getElementById('btn-center'),
  
  // Sliders
  sliderX: document.getElementById('slider-x'),
  sliderY: document.getElementById('slider-y'),
  sliderValX: document.getElementById('slider-val-x'),
  sliderValY: document.getElementById('slider-val-y'),
  
  // Joystick
  joystickPad: document.getElementById('joystick-pad'),
  joystickHandle: document.getElementById('joystick-handle'),
  
  // Terminal Logs
  logTerminal: document.getElementById('log-terminal'),
  clearLogBtn: document.getElementById('clear-log-btn')
};

// Canvas 2D Context for Simulation Rendering
const ctx = elements.simulationCanvas.getContext('2d');
let simulationAnimationId = null;

// Initialize
function init() {
  setupEventListeners();
  checkTurboPiStatus();
  updateUI();
  startSimulationLoop();
}

// -------------------------------------------------------------
// UI Updates & Terminal Logging
// -------------------------------------------------------------

function logToTerminal(message, type = 'system') {
  const timestamp = new Date().toLocaleTimeString();
  const line = document.createElement('div');
  line.className = `log-line ${type}-line`;
  
  let prefix = '[SYS]';
  if (type === 'sent') prefix = '[OUT]';
  if (type === 'received') prefix = '[IN]';
  if (type === 'error') prefix = '[ERR]';
  
  line.textContent = `${timestamp} ${prefix} ${message}`;
  elements.logTerminal.appendChild(line);
  elements.logTerminal.scrollTop = elements.logTerminal.scrollHeight;
  
  // Keep logs to a reasonable length
  while (elements.logTerminal.children.length > 50) {
    elements.logTerminal.removeChild(elements.logTerminal.firstChild);
  }
}

function updateUI() {
  // Update coordinate displays
  elements.valX.textContent = state.x;
  elements.valY.textContent = state.y;
  
  // Update slider positions
  elements.sliderX.value = state.x;
  elements.sliderY.value = state.y;
  elements.sliderValX.textContent = `${state.x}°`;
  elements.sliderValY.textContent = `${state.y}°`;
  
  // Manage Video Stream visibility
  if (state.isSimulation) {
    elements.cameraStream.classList.add('hidden');
    elements.simulationCanvas.classList.remove('hidden');
    elements.streamOverlayError.classList.add('hidden');
    elements.streamTypeLabel.textContent = 'Simulation';
    elements.streamSourceLabel.textContent = 'Canvas Mock';
  } else {
    elements.simulationCanvas.classList.add('hidden');
    elements.cameraStream.classList.remove('hidden');
    elements.streamTypeLabel.textContent = 'Proxy Stream';
    elements.streamSourceLabel.textContent = `${window.location.host}/api/stream`;
  }
}

// -------------------------------------------------------------
// Connection & Hardware Sync
// -------------------------------------------------------------

async function checkTurboPiStatus() {
  state.isConnecting = true;
  updateConnectionStatusUI('connecting');
  
  try {
    // Send a minimal request to test JSON-RPC availability
    const response = await fetch('/api/rpc', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ method: 'GetPWMServo', params: [] }) // Or any light method
    });
    
    if (response.ok) {
      logToTerminal('TurboPi hardware responded. Online.');
      state.isSimulation = false;
      elements.simulationToggle.checked = false;
      updateConnectionStatusUI('online');
      
      // Sync initial position with TurboPi
      sendServoPositions(true);
    } else {
      throw new Error('Non-ok response');
    }
  } catch (error) {
    logToTerminal('TurboPi unreachable. Falling back to Simulation Mode.', 'error');
    state.isSimulation = true;
    elements.simulationToggle.checked = true;
    updateConnectionStatusUI('offline');
  } finally {
    state.isConnecting = false;
    updateUI();
  }
}

function updateConnectionStatusUI(status) {
  const dot = elements.connectionStatus.querySelector('.status-dot');
  const text = elements.connectionStatus.querySelector('.status-text');
  
  dot.className = 'status-dot';
  
  if (status === 'online') {
    dot.classList.add('online');
    text.textContent = 'TurboPi Online';
  } else if (status === 'offline') {
    dot.classList.add('offline');
    text.textContent = 'TurboPi Offline';
  } else {
    dot.classList.add('pinging');
    text.textContent = 'Connecting...';
  }
}

// Send servo instructions via API
async function sendServoPositions(force = false) {
  // Prevent redundant calls if position has not changed (unless forced)
  if (!force && state.x === state.lastSentX && state.y === state.lastSentY) {
    return;
  }
  
  state.lastSentX = state.x;
  state.lastSentY = state.y;
  
  // Format matching controller/main.py: SetPWMServo with values [TIME_MS, 2, 1, x, 2, y]
  const method = 'SetPWMServo';
  const params = [TIME_MS, 2, 1, state.x, 2, state.y];
  
  if (state.isSimulation) {
    logToTerminal(`SIMULATED SetPWMServo: TIME=${TIME_MS}ms, Servo1(Pitch)=${state.x}°, Servo2(Yaw)=${state.y}°`, 'sent');
    return;
  }
  
  logToTerminal(`Call: ${method}(${params.join(', ')})`, 'sent');
  
  try {
    const response = await fetch('/api/rpc', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ method, params })
    });
    
    const result = await response.json();
    
    if (response.ok && !result.error) {
      logToTerminal(`Response: ${JSON.stringify(result.result)}`, 'received');
      updateConnectionStatusUI('online');
    } else {
      const errorMsg = result.error ? result.error.message || JSON.stringify(result.error) : 'Unknown error';
      logToTerminal(`RPC Error: ${errorMsg}`, 'error');
    }
  } catch (error) {
    logToTerminal(`Network Error: ${error.message}`, 'error');
    updateConnectionStatusUI('offline');
  }
}

// Send individual servo update if only one changes (fine tuning)
async function sendSingleServoPosition(servoId, angle) {
  const method = 'SetPWMServo';
  const params = [TIME_MS, 1, servoId, angle];
  
  if (state.isSimulation) {
    logToTerminal(`SIMULATED SetPWMServo: TIME=${TIME_MS}ms, Servo${servoId}=${angle}°`, 'sent');
    return;
  }
  
  logToTerminal(`Call: ${method}(${params.join(', ')})`, 'sent');
  
  try {
    const response = await fetch('/api/rpc', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ method, params })
    });
    
    const result = await response.json();
    if (response.ok && !result.error) {
      logToTerminal(`Response: ${JSON.stringify(result.result)}`, 'received');
    } else {
      logToTerminal(`RPC Error: ${result.error?.message || 'Error'}`, 'error');
    }
  } catch (error) {
    logToTerminal(`Network Error: ${error.message}`, 'error');
  }
}

// -------------------------------------------------------------
// Interactive Navigation Control
// -------------------------------------------------------------

function setPosition(x, y) {
  state.x = Math.max(SERVO_MIN, Math.min(SERVO_MAX, Math.round(x)));
  state.y = Math.max(SERVO_MIN, Math.min(SERVO_MAX, Math.round(y)));
  updateUI();
  sendServoPositions();
}

function adjustPosition(dx, dy) {
  setPosition(state.x + dx, state.y + dy);
}

function resetPosition() {
  logToTerminal('Resetting camera to default coordinates (X=10, Y=10)...');
  setPosition(DEFAULT_X, DEFAULT_Y);
}

// Keyboard shortcuts mapping
function handleKeyDown(e) {
  // Prevent default scroll actions for arrow keys and space when focus is in app
  if (['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight', ' '].includes(e.key)) {
    e.preventDefault();
  }
  
  switch (e.key) {
    case 'ArrowUp':
      adjustPosition(state.stepSize, 0); // Up increases Pitch (X)
      animateButton(elements.btnUp);
      break;
    case 'ArrowDown':
      adjustPosition(-state.stepSize, 0); // Down decreases Pitch (X)
      animateButton(elements.btnDown);
      break;
    case 'ArrowLeft':
      adjustPosition(0, -state.stepSize); // Left decreases Yaw (Y)
      animateButton(elements.btnLeft);
      break;
    case 'ArrowRight':
      adjustPosition(0, state.stepSize); // Right increases Yaw (Y)
      animateButton(elements.btnRight);
      break;
    case ' ':
      resetPosition();
      animateButton(elements.btnCenter);
      break;
  }
}

function animateButton(btn) {
  btn.classList.add('active');
  setTimeout(() => btn.classList.remove('active'), 100);
}

// -------------------------------------------------------------
// Virtual Joystick Control Logic
// -------------------------------------------------------------

function setupJoystick() {
  const pad = elements.joystickPad;
  const handle = elements.joystickHandle;
  
  let isDragging = false;
  let padRect = null;
  let center = { x: 0, y: 0 };
  let maxDistance = 45; // Max radius handle can move from center
  
  let updateInterval = null;
  let joystickInput = { x: 0, y: 0 }; // Values from -1.0 to 1.0

  pad.addEventListener('mousedown', startDrag);
  pad.addEventListener('touchstart', startDrag, { passive: false });
  
  window.addEventListener('mousemove', drag);
  window.addEventListener('touchmove', drag, { passive: false });
  
  window.addEventListener('mouseup', endDrag);
  window.addEventListener('touchend', endDrag);

  function startDrag(e) {
    isDragging = true;
    padRect = pad.getBoundingClientRect();
    center = {
      x: padRect.width / 2,
      y: padRect.height / 2
    };
    
    // Set active styles
    pad.style.borderColor = 'rgba(139, 92, 246, 0.5)';
    
    // Start continuous update timer (every 100ms) to steer camera while holding joystick
    updateInterval = setInterval(updateFromJoystick, 100);
    
    drag(e);
  }

  function drag(e) {
    if (!isDragging) return;
    e.preventDefault();
    
    const clientX = e.touches ? e.touches[0].clientX : e.clientX;
    const clientY = e.touches ? e.touches[0].clientY : e.clientY;
    
    // Position relative to pad center
    let dx = clientX - (padRect.left + center.x);
    let dy = clientY - (padRect.top + center.y);
    
    const distance = Math.sqrt(dx * dx + dy * dy);
    
    if (distance > maxDistance) {
      dx = (dx / distance) * maxDistance;
      dy = (dy / distance) * maxDistance;
    }
    
    // Position handle
    handle.style.transform = `translate(${dx}px, ${dy}px)`;
    
    // Normalize input to [-1.0, 1.0] range
    // Pitch (up/down) matches y-axis, Yaw (left/right) matches x-axis
    joystickInput.x = dx / maxDistance;
    joystickInput.y = -dy / maxDistance; // Invert to match Up=positive, Down=negative
  }

  function endDrag() {
    if (!isDragging) return;
    isDragging = false;
    
    clearInterval(updateInterval);
    updateInterval = null;
    
    // Return handle to center
    handle.style.transform = 'translate(0px, 0px)';
    pad.style.borderColor = 'rgba(255, 255, 255, 0.1)';
    
    joystickInput = { x: 0, y: 0 };
  }

  function updateFromJoystick() {
    if (Math.abs(joystickInput.x) < 0.1 && Math.abs(joystickInput.y) < 0.1) return;
    
    // Adjust Pitch (X) based on vertical joystick position, and Yaw (Y) based on horizontal
    // Speed is proportional to joystick deflection and current step size
    const speedMultiplier = 1.5;
    const dx = joystickInput.y * state.stepSize * speedMultiplier;
    const dy = joystickInput.x * state.stepSize * speedMultiplier;
    
    adjustPosition(dx, dy);
  }
}

// -------------------------------------------------------------
// Canvas Simulation Renderer
// -------------------------------------------------------------

function startSimulationLoop() {
  if (simulationAnimationId) {
    cancelAnimationFrame(simulationAnimationId);
  }
  
  // Set internal resolution
  elements.simulationCanvas.width = 640;
  elements.simulationCanvas.height = 480;
  
  let scanlineOffset = 0;
  let particleList = Array.from({ length: 15 }, () => ({
    x: Math.random() * 640,
    y: Math.random() * 480,
    size: Math.random() * 2 + 1,
    speed: Math.random() * 0.5 + 0.2
  }));

  function draw() {
    if (!state.isSimulation) {
      simulationAnimationId = requestAnimationFrame(draw);
      return;
    }

    // Clear
    ctx.fillStyle = '#06020f';
    ctx.fillRect(0, 0, 640, 480);
    
    // Draw Grid lines
    ctx.strokeStyle = 'rgba(139, 92, 246, 0.08)';
    ctx.lineWidth = 1;
    const gridSize = 40;
    for (let x = 0; x < 640; x += gridSize) {
      ctx.beginPath();
      ctx.moveTo(x, 0);
      ctx.lineTo(x, 480);
      ctx.stroke();
    }
    for (let y = 0; y < 480; y += gridSize) {
      ctx.beginPath();
      ctx.moveTo(0, y);
      ctx.lineTo(640, y);
      ctx.stroke();
    }

    // Draw cyber background particles
    ctx.fillStyle = 'rgba(139, 92, 246, 0.2)';
    particleList.forEach(p => {
      ctx.beginPath();
      ctx.arc(p.x, p.y, p.size, 0, Math.PI * 2);
      ctx.fill();
      p.y -= p.speed;
      if (p.y < 0) {
        p.y = 480;
        p.x = Math.random() * 640;
      }
    });

    // Map X, Y camera angles to coordinate system
    // Pitch (X) -> controls Vertical position (Up is high angle, which means higher on screen)
    // Yaw (Y) -> controls Horizontal position (Right is high angle, which is right on screen)
    // Scale: angle SERVO_MIN (0) to SERVO_MAX (180) mapped to coords
    const margin = 60;
    const viewWidth = 640 - margin * 2;
    const viewHeight = 480 - margin * 2;
    
    // Calculate normalized percentage (0.0 to 1.0)
    const pctY = (state.y - SERVO_MIN) / (SERVO_MAX - SERVO_MIN);
    // Pitch: higher angle = camera looks up. So y-coord should be higher (smaller screen y)
    const pctX = 1 - (state.x - SERVO_MIN) / (SERVO_MAX - SERVO_MIN); 
    
    // Reticle targets
    const targetX = margin + pctY * viewWidth;
    const targetY = margin + pctX * viewHeight;
    
    // Reticle rendering (cool glowing camera target crosshair)
    ctx.shadowBlur = 15;
    ctx.shadowColor = '#10b981';
    
    ctx.strokeStyle = 'rgba(16, 185, 129, 0.4)';
    ctx.lineWidth = 2;
    
    // Outer dashed circle
    ctx.beginPath();
    ctx.arc(targetX, targetY, 40, 0, Math.PI * 2);
    ctx.setLineDash([6, 8]);
    ctx.stroke();
    ctx.setLineDash([]);
    
    // Inner solid circle
    ctx.strokeStyle = '#10b981';
    ctx.beginPath();
    ctx.arc(targetX, targetY, 8, 0, Math.PI * 2);
    ctx.stroke();
    
    // Center point
    ctx.fillStyle = '#10b981';
    ctx.beginPath();
    ctx.arc(targetX, targetY, 2, 0, Math.PI * 2);
    ctx.fill();

    // Crosshair ticks
    ctx.beginPath();
    // Up
    ctx.moveTo(targetX, targetY - 15);
    ctx.lineTo(targetX, targetY - 25);
    // Down
    ctx.moveTo(targetX, targetY + 15);
    ctx.lineTo(targetX, targetY + 25);
    // Left
    ctx.moveTo(targetX - 15, targetY);
    ctx.lineTo(targetX - 25, targetY);
    // Right
    ctx.moveTo(targetX + 15, targetY);
    ctx.lineTo(targetX + 25, targetY);
    ctx.stroke();
    
    ctx.shadowBlur = 0; // Reset shadow

    // Text info on HUD
    ctx.font = '11px "JetBrains Mono", monospace';
    ctx.fillStyle = 'rgba(255, 255, 255, 0.7)';
    ctx.fillText(`SIMULATED FEED`, 24, 35);
    
    ctx.fillStyle = '#10b981';
    ctx.fillText(`PITCH (SERVO 1): ${state.x}°`, 24, 430);
    ctx.fillText(`YAW   (SERVO 2): ${state.y}°`, 24, 450);
    
    ctx.fillStyle = 'rgba(255, 255, 255, 0.4)';
    ctx.fillText(`PAN REGION: [${SERVO_MIN}° - ${SERVO_MAX}°]`, 460, 450);

    // Scanline overlay effect
    ctx.fillStyle = 'rgba(255, 255, 255, 0.015)';
    for (let i = 0; i < 480; i += 4) {
      ctx.fillRect(0, (i + scanlineOffset) % 480, 640, 2);
    }
    scanlineOffset = (scanlineOffset + 0.5) % 480;

    // Corner HUD brackets
    ctx.strokeStyle = 'rgba(255, 255, 255, 0.15)';
    ctx.lineWidth = 2;
    const len = 20;
    // Top-Left
    ctx.beginPath(); ctx.moveTo(15, 15 + len); ctx.lineTo(15, 15); ctx.lineTo(15 + len, 15); ctx.stroke();
    // Top-Right
    ctx.beginPath(); ctx.moveTo(625, 15 + len); ctx.lineTo(625, 15); ctx.lineTo(625 - len, 15); ctx.stroke();
    // Bottom-Left
    ctx.beginPath(); ctx.moveTo(15, 465 - len); ctx.lineTo(15, 465); ctx.lineTo(15 + len, 465); ctx.stroke();
    // Bottom-Right
    ctx.beginPath(); ctx.moveTo(625, 465 - len); ctx.lineTo(625, 465); ctx.lineTo(625 - len, 465); ctx.stroke();

    simulationAnimationId = requestAnimationFrame(draw);
  }
  
  draw();
}

// -------------------------------------------------------------
// Event Handlers Setup
// -------------------------------------------------------------

function setupEventListeners() {
  // Keypress event listener
  window.addEventListener('keydown', handleKeyDown);

  // Simulation Mode Toggle
  elements.simulationToggle.addEventListener('change', (e) => {
    state.isSimulation = e.target.checked;
    logToTerminal(`Simulation Mode changed to ${state.isSimulation ? 'ENABLED' : 'DISABLED'}`);
    
    if (!state.isSimulation) {
      // Reload stream source
      elements.cameraStream.src = '/api/stream?t=' + Date.now();
      checkTurboPiStatus();
    }
    updateUI();
  });

  // Reconnect stream handler
  elements.reconnectBtn.addEventListener('click', () => {
    logToTerminal('Attempting to reconnect hardware...');
    checkTurboPiStatus();
  });

  elements.refreshStreamBtn.addEventListener('click', () => {
    logToTerminal('Refreshing camera stream cache...');
    elements.cameraStream.src = '/api/stream?t=' + Date.now();
  });

  elements.fullscreenBtn.addEventListener('click', () => {
    const stream = state.isSimulation ? elements.simulationCanvas : elements.cameraStream;
    if (stream.requestFullscreen) {
      stream.requestFullscreen();
    } else if (stream.webkitRequestFullscreen) { /* Safari */
      stream.webkitRequestFullscreen();
    } else if (stream.msRequestFullscreen) { /* IE11 */
      stream.msRequestFullscreen();
    }
  });

  // D-Pad Click Events
  elements.btnUp.addEventListener('click', () => adjustPosition(state.stepSize, 0));
  elements.btnDown.addEventListener('click', () => adjustPosition(-state.stepSize, 0));
  elements.btnLeft.addEventListener('click', () => adjustPosition(0, -state.stepSize));
  elements.btnRight.addEventListener('click', () => adjustPosition(0, state.stepSize));
  elements.btnCenter.addEventListener('click', resetPosition);

  // Range Sliders
  elements.sliderX.addEventListener('input', (e) => {
    const val = parseInt(e.target.value);
    state.x = val;
    elements.sliderValX.textContent = `${val}°`;
    elements.valX.textContent = val;
  });
  
  elements.sliderX.addEventListener('change', (e) => {
    sendSingleServoPosition(1, state.x);
  });

  elements.sliderY.addEventListener('input', (e) => {
    const val = parseInt(e.target.value);
    state.y = val;
    elements.sliderValY.textContent = `${val}°`;
    elements.valY.textContent = val;
  });
  
  elements.sliderY.addEventListener('change', (e) => {
    sendSingleServoPosition(2, state.y);
  });

  // Step selector
  elements.stepSizeSelect.addEventListener('change', (e) => {
    state.stepSize = parseInt(e.target.value);
    logToTerminal(`Control step size changed to ${state.stepSize}°`);
  });

  // Reset Button
  elements.resetBtn.addEventListener('click', resetPosition);

  // Terminal Clear button
  elements.clearLogBtn.addEventListener('click', () => {
    elements.logTerminal.innerHTML = '';
    logToTerminal('Terminal cleared.');
  });

  // Real-time error handler for proxy stream loading
  elements.cameraStream.addEventListener('error', () => {
    if (!state.isSimulation) {
      elements.streamOverlayError.classList.remove('hidden');
    }
  });

  elements.cameraStream.addEventListener('load', () => {
    elements.streamOverlayError.classList.add('hidden');
  });

  // Setup virtual joystick
  setupJoystick();
}

// Start Webapp client
document.addEventListener('DOMContentLoaded', init);
