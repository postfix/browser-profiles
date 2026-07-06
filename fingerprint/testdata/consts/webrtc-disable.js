(function() {
  'use strict';
  
  // Disable WebRTC by removing the global constructor to prevent host IP leaks.
  try {
    window.RTCPeerConnection = undefined;
    window.webkitRTCPeerConnection = undefined;
  } catch (e) {}
  
  try {
    delete window.RTCPeerConnection;
    delete window.webkitRTCPeerConnection;
  } catch (e) {}
  
  console.log('[browser-profiles] WebRTC disabled');
})();
