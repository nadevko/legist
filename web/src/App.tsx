import { createBrowserRouter, RouterProvider, Navigate } from 'react-router-dom'
import AuthPage from './pages/AuthPage'
import NotFoundPage from './pages/NotFoundPage'
import { AppLayout, AuthGuard } from './components/layout/AppLayout'
import { ActsPage } from './pages/ActsPage'
import { ActDetailPage, HomePage, ComparePage, ChainPage, AssistantPage } from './pages/index'

const router = createBrowserRouter([
  {
    path: '/auth',
    element: <AuthPage/>,
  },
  {
    element: <AuthGuard/>,
    children: [
      {
        element: <AppLayout/>,
        children: [
          { index: true,            element: <HomePage/> },
          { path: 'acts',           element: <ActsPage/> },
          { path: 'acts/:id',       element: <ActDetailPage/> },
          { path: 'compare/:jobId', element: <ComparePage/> },
          { path: 'chain/:actId',   element: <ChainPage/> },
          { path: 'assistant',      element: <AssistantPage/> },
          { path: '*',              element: <NotFoundPage/> },
        ],
      },
    ],
  },
])

export default function App() {
  return <RouterProvider router={router}/>
}
