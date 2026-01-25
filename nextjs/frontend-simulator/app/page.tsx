export default function Home() {
  const timestamp = new Date().toISOString();

  return (
    <div className="flex min-h-screen items-center justify-center bg-gradient-to-br from-blue-50 to-indigo-100 dark:from-gray-900 dark:to-gray-800">
      <main className="max-w-2xl mx-auto p-8">
        <div className="bg-white dark:bg-gray-800 rounded-lg shadow-xl p-8">
          <h1 className="text-4xl font-bold text-gray-900 dark:text-white mb-4">
            Frontend Simulator
          </h1>
          <p className="text-lg text-gray-600 dark:text-gray-300 mb-6">
            Running on Next.js
          </p>

          <div className="bg-gray-50 dark:bg-gray-700 rounded-lg p-6 mb-6">
            <div className="flex items-center mb-2">
              <span className="inline-block w-3 h-3 bg-green-500 rounded-full mr-3"></span>
              <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">Status: Running</span>
            </div>
            <p className="text-sm text-gray-500 dark:text-gray-400 ml-6">
              Timestamp: {timestamp}
            </p>
          </div>

          <div className="space-y-4">
            <h2 className="text-xl font-semibold text-gray-800 dark:text-white">
              Available Endpoints:
            </h2>
            <ul className="space-y-2 text-sm text-gray-600 dark:text-gray-300">
              <li className="flex items-start">
                <code className="bg-gray-100 dark:bg-gray-600 px-2 py-1 rounded text-xs font-mono mr-2">GET</code>
                <span>/api/status</span>
              </li>
              <li className="flex items-start">
                <code className="bg-gray-100 dark:bg-gray-600 px-2 py-1 rounded text-xs font-mono mr-2">POST</code>
                <span>/api/data</span>
              </li>
              <li className="flex items-start">
                <code className="bg-gray-100 dark:bg-gray-600 px-2 py-1 rounded text-xs font-mono mr-2">GET</code>
                <span>/api/health</span>
              </li>
            </ul>
          </div>
        </div>
      </main>
    </div>
  );
}
